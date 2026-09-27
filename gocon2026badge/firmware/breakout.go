package main

import (
	"machine"

	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/st7789"
)

// 全画面ブロック崩しのムービー画面。操作は無く、勝手に流れ続ける。
// 1 マス 3x3px の 80x80 マス。ほぼ全面が壊せるブロックで、
// 下部のスポーン部屋から通路を通って中央のアイテムに球が届くと
// 球が一気に bkMaxBalls 個へ増殖する。
const (
	bkGrid     = 80
	bkCellPx   = 3
	bkUnit     = bkCellPx * 16 // 1 マスの内部座標 (1/16px 固定小数)
	bkFieldMax = bkGrid * bkUnit
	bkMaxBalls     = 150 // 全アイテム取得時の最大球数
	bkBallsPerItem = 30  // アイテム 1 つで増える球数
	bkResetAt      = 200 // 残りブロックがこの数以下になったらリセットへ (終盤の間延び防止)
	bkResetWt      = 300 // リセット条件成立からリセットまでの待ち (30Hz x 10 秒)
	bkWipePhase    = 30  // 幕引きの片道フレーム数 (8px/フレーム x 30 = 240px ≈ 1 秒)
)

// 増殖アイテム (4x4 マス) の左上座標。中央 + 四隅寄り
var bkItems = [5][2]int{
	{38, 38},
	{8, 8}, {68, 8},
	{8, 68}, {68, 68},
}

const (
	bkEmpty uint8 = iota
	bkBlock
	bkItem
)

type bkBall struct {
	x, y   int // 中心座標 (1/16px)
	vx, vy int // 速度 (1/16px / フレーム)
}

var (
	bkField   [bkGrid][bkGrid]uint8
	bkBalls   [bkMaxBalls]bkBall
	bkNumBall = 0
	bkBlocks  = 0 // 残りの壊せるマス数 (アイテム含む)
	bkResetIn = 0 // 0 以外ならリセット待ち
	bkWipe    = 0 // 0 以外なら幕引き中 (1..bkWipePhase: 降下, ..bkWipePhase*2: 抜け)
	bkFrame   = 0
)

// おおよそ等速 (≈72/16px/フレーム) の 16 方向。軸平行は避ける
var bkDirs = [16][2]int{
	{70, 19}, {59, 41}, {41, 59}, {19, 70},
	{-19, 70}, {-41, 59}, {-59, 41}, {-70, 19},
	{-70, -19}, {-59, -41}, {-41, -59}, {-19, -70},
	{19, -70}, {41, -59}, {59, -41}, {70, -19},
}

// 10 行ごとの帯の色 (上から虹色)。RGB565BE の生値
var bkBandColors = [8]pixel.RGB565BE{
	pixel.NewColor[pixel.RGB565BE](0xE0, 0x40, 0x40), // 赤
	pixel.NewColor[pixel.RGB565BE](0xE0, 0x90, 0x30), // 橙
	pixel.NewColor[pixel.RGB565BE](0xD8, 0xD0, 0x30), // 黄
	pixel.NewColor[pixel.RGB565BE](0x40, 0xC0, 0x50), // 緑
	pixel.NewColor[pixel.RGB565BE](0x00, 0xAD, 0xD8), // Go ブルー
	pixel.NewColor[pixel.RGB565BE](0x40, 0x60, 0xE0), // 青
	pixel.NewColor[pixel.RGB565BE](0x90, 0x50, 0xE0), // 紫
	pixel.NewColor[pixel.RGB565BE](0xE0, 0x60, 0xA0), // 桃
}

var (
	bkColEmpty   = pixel.NewColor[pixel.RGB565BE](0x00, 0x08, 0x10)
	bkColItem1   = pixel.NewColor[pixel.RGB565BE](0xFF, 0xE0, 0x40) // アイテム (点滅 1)
	bkColItem2   = pixel.NewColor[pixel.RGB565BE](0xFF, 0xFF, 0xFF) // アイテム (点滅 2)
	bkColBall    = pixel.NewColor[pixel.RGB565BE](0xFF, 0xFF, 0xFF)
	bkColCurtain = pixel.NewColor[pixel.RGB565BE](0x10, 0x10, 0x18) // 幕
	bkColEdge    = pixel.NewColor[pixel.RGB565BE](0x00, 0xAD, 0xD8) // 幕の縁
)

// 軽量な擬似乱数 (xorshift32)。初回にハードウェア乱数でシードする
var bkRand uint32 = 0

func bkRnd() uint32 {
	if bkRand == 0 {
		if v, err := machine.GetRNG(); err == nil && v != 0 {
			bkRand = v
		} else {
			bkRand = 0x6042_2026
		}
	}
	x := bkRand
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	bkRand = x
	return x
}

// bkJitter は反射した球の速度成分をたまに ±1 だけ変え、
// 毎回同じ軌道の繰り返しに見えないようにする
func bkJitter(v *int) {
	r := bkRnd()
	if r&3 != 0 { // 1/4 の確率でだけ効かせる
		return
	}
	d := 1
	if r&4 != 0 {
		d = -1
	}
	nv := *v + d
	if nv > -16 && nv < 16 { // 遅くなりすぎない
		return
	}
	if nv > 78 || nv < -78 { // 速くなりすぎない (1 マス跳び防止)
		return
	}
	*v = nv
}

// bkInit は初期マップを作る (マップは固定、球の初速だけ乱数で揺らす)
func bkInit() {
	bkBlocks = 0
	for y := 0; y < bkGrid; y++ {
		for x := 0; x < bkGrid; x++ {
			bkField[y][x] = bkBlock
			bkBlocks++
		}
	}
	carve := func(x0, y0, x1, y1 int, v uint8) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if bkField[y][x] != bkEmpty {
					bkBlocks--
				}
				bkField[y][x] = v
				if v != bkEmpty {
					bkBlocks++
				}
			}
		}
	}
	// (x0,y0) から (x1,y1) へ 3 マス幅の通路を掘る
	carvePath := func(x0, y0, x1, y1 int) {
		steps := x1 - x0
		if steps < 0 {
			steps = -steps
		}
		if d := y1 - y0; d > steps {
			steps = d
		} else if -d > steps {
			steps = -d
		}
		for i := 0; i <= steps; i++ {
			x := x0 + (x1-x0)*i/steps
			y := y0 + (y1-y0)*i/steps
			carve(x-1, y-1, x+1, y+1, bkEmpty)
		}
	}

	carve(36, 70, 43, 77, bkEmpty) // 下部中央のスポーン部屋
	// 中央アイテムへ続く通路 (煙突)。部屋との間に 2 行の壁を残し、
	// 最初の球がしばらく部屋で暴れてから掘り抜いて入る (導入 ≈ 8 秒)
	carve(39, 42, 40, 67, bkEmpty)

	// 中央チェンバーの四隅から各隅のアイテムへ向かう斜めの導入通路。
	// アイテムの手前 1 マスで止め、球が壁を掘り抜く「タメ」を作る
	carvePath(37, 37, 14, 14)
	carvePath(42, 37, 65, 14)
	carvePath(37, 42, 14, 65)
	carvePath(42, 42, 65, 65)

	for _, it := range bkItems {
		carve(it[0], it[1], it[0]+3, it[1]+3, bkItem)
	}

	// 斜めの初速: しばらく部屋で暴れてから煙突に入り、アイテムへ。
	// 速度と左右の向きに揺らぎを入れて毎回違う導入にする
	vx := 55 + int(bkRnd()%10) // 55..64
	vy := 37 + int(bkRnd()%10) // 37..46
	if bkRnd()&1 != 0 {
		vx = -vx
	}
	bkNumBall = 1
	bkBalls[0] = bkBall{
		x:  40*bkUnit + bkUnit/2,
		y:  74*bkUnit + bkUnit/2,
		vx: vx, vy: -vy,
	}
	bkResetIn = 0
	bkFrame = 0
}

// bkMultiball はアイテム取得時の球の増殖。
// ヒットしたマスを含むアイテム (4x4) だけを消し、bkBallsPerItem 個増やす
func bkMultiball(cx, cy, px, py int) {
	for _, it := range bkItems {
		if cx < it[0] || cx > it[0]+3 || cy < it[1] || cy > it[1]+3 {
			continue
		}
		for y := it[1]; y <= it[1]+3; y++ {
			for x := it[0]; x <= it[0]+3; x++ {
				if bkField[y][x] == bkItem {
					bkField[y][x] = bkEmpty
					bkBlocks--
				}
			}
		}
		break
	}
	target := bkNumBall + bkBallsPerItem
	if target > bkMaxBalls {
		target = bkMaxBalls
	}
	i := int(bkRnd() % uint32(len(bkDirs))) // 放射方向の開始位置をランダムに
	for bkNumBall < target {
		d := bkDirs[i%len(bkDirs)]
		scale := 14 + i%5 // 速度に少し個体差をつける (x14/16 〜 x18/16)
		bkBalls[bkNumBall] = bkBall{
			x: px, y: py,
			vx: d[0] * scale / 16,
			vy: d[1] * scale / 16,
		}
		bkNumBall++
		i++
	}
}

// bkDestroy はマスを壊す。アイテムなら増殖を起こす
func bkDestroy(cx, cy, px, py int) {
	if bkField[cy][cx] == bkItem {
		bkMultiball(cx, cy, px, py)
		return
	}
	bkField[cy][cx] = bkEmpty
	bkBlocks--
	if bkBlocks <= bkResetAt && bkResetIn == 0 && bkWipe == 0 {
		bkResetIn = bkResetWt
	}
}

// bkStep は物理を 1 フレーム進める (軸ごとの衝突判定)
func bkStep() {
	bkFrame++
	if bkWipe > 0 {
		bkWipe++
		if bkWipe == bkWipePhase {
			// 幕が画面を覆いきったところで新しい盤面に差し替える
			// (bkInit は bkWipe を触らないので、続けて幕が抜けて新盤面が現れる)
			bkInit()
		}
		if bkWipe >= bkWipePhase*2 {
			bkWipe = 0
		}
	} else if bkResetIn > 0 {
		bkResetIn--
		if bkResetIn == 0 {
			bkWipe = 1
		}
	}
	const lo = bkUnit / 2
	const hi = bkFieldMax - bkUnit/2
	for i := 0; i < bkNumBall; i++ {
		b := &bkBalls[i]

		// 1 サブステップの移動量が 1 マス (48) 未満になるよう分割し、
		// セル境界を一度に 2 つ跨ぐ「すり抜け」を防ぐ
		m := b.vx
		if m < 0 {
			m = -m
		}
		if v := b.vy; v > m {
			m = v
		} else if -v > m {
			m = -v
		}
		n := m/bkUnit + 1

		for s := 0; s < n; s++ {
			nx := b.x + b.vx/n
			if nx < lo || nx > hi {
				b.vx = -b.vx
			} else if c := bkField[b.y/bkUnit][nx/bkUnit]; c != bkEmpty {
				bkDestroy(nx/bkUnit, b.y/bkUnit, b.x, b.y)
				b.vx = -b.vx
				bkJitter(&b.vy)
			} else {
				b.x = nx
			}

			ny := b.y + b.vy/n
			if ny < lo || ny > hi {
				b.vy = -b.vy
			} else if c := bkField[ny/bkUnit][b.x/bkUnit]; c != bkEmpty {
				bkDestroy(b.x/bkUnit, ny/bkUnit, b.x, b.y)
				b.vy = -b.vy
				bkJitter(&b.vx)
			} else {
				b.y = ny
			}
		}
	}
}

// bkRender は盤面と球を pixelBuf へ描く (1 マス = 3x3px)
func bkRender() {
	raw := pixelBuf.RawBuffer()

	itemCol := bkColItem1
	if bkFrame&8 != 0 {
		itemCol = bkColItem2
	}

	for cy := 0; cy < bkGrid; cy++ {
		base := cy * bkCellPx * 480
		band := bkBandColors[cy/10]
		for cx := 0; cx < bkGrid; cx++ {
			var c pixel.RGB565BE
			switch bkField[cy][cx] {
			case bkBlock:
				c = band
			case bkItem:
				c = itemCol
			default:
				c = bkColEmpty
			}
			o := base + cx*bkCellPx*2
			l, h := byte(c), byte(c>>8)
			raw[o], raw[o+1] = l, h
			raw[o+2], raw[o+3] = l, h
			raw[o+4], raw[o+5] = l, h
		}
		// 1 ピクセル行を作って残り 2 行へコピー
		copy(raw[base+480:base+960], raw[base:base+480])
		copy(raw[base+960:base+1440], raw[base:base+480])
	}

	// 球 (3x3px の白)
	l, h := byte(bkColBall), byte(bkColBall>>8)
	for i := 0; i < bkNumBall; i++ {
		px := bkBalls[i].x/16 - 1
		py := bkBalls[i].y/16 - 1
		if px < 0 {
			px = 0
		}
		if px > 237 {
			px = 237
		}
		if py < 0 {
			py = 0
		}
		if py > 237 {
			py = 237
		}
		for r := 0; r < 3; r++ {
			o := (py+r)*480 + px*2
			raw[o], raw[o+1] = l, h
			raw[o+2], raw[o+3] = l, h
			raw[o+4], raw[o+5] = l, h
		}
	}

	// 幕引き: 前半は幕の下端が降りて盤面を覆い、後半は上端が降りて新盤面を現す
	if bkWipe > 0 {
		top, bottom := 0, bkWipe*8
		edgeAtBottom := true
		if bkWipe > bkWipePhase {
			top, bottom = (bkWipe-bkWipePhase)*8, 240
			edgeAtBottom = false
		}
		if bottom > 240 {
			bottom = 240
		}
		if top < bottom {
			fillRows := func(y0, y1 int, c pixel.RGB565BE) {
				cl, ch := byte(c), byte(c>>8)
				base := y0 * 480
				for x := 0; x < 480; x += 2 {
					raw[base+x], raw[base+x+1] = cl, ch
				}
				for y := y0 + 1; y < y1; y++ {
					copy(raw[y*480:(y+1)*480], raw[base:base+480])
				}
			}
			fillRows(top, bottom, bkColCurtain)
			// 動いている側の縁 4px を Go ブルーで
			if edgeAtBottom {
				e0 := bottom - 4
				if e0 < top {
					e0 = top
				}
				fillRows(e0, bottom, bkColEdge)
			} else {
				e1 := top + 4
				if e1 > bottom {
					e1 = bottom
				}
				fillRows(top, e1, bkColEdge)
			}
		}
	}
}

// updateBreakout は 30Hz で呼ばれ、1 フレーム進めて全画面を転送する
func updateBreakout(display st7789.Device) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	bkStep()
	bkRender()
	return display.DrawBitmap(0, 0, pixelBuf)
}
