package main

import (
	"image/color"

	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
)

// 疑似 3D デモ画面。操作なしで 2 つのフェーズをループする:
//  1. スターフィールド (ワープ)
//  2. + 星が集まって "Go Conference 2026" を組み、静止してから弾け飛ぶ
const (
	dmNumStars = 160

	dmPhaseStars1 = 240 // 8 秒
	dmPhaseText   = 330 // 11 秒 (集合 2 秒 + 静止 6 秒 + 拡散 3 秒)
	dmPhaseTotal  = dmPhaseStars1 + dmPhaseText

	dmTextGather  = 60  // 集合にかけるフレーム数
	dmTextScatter = 240 // このフレームから拡散
)

type dmStar struct{ x, y, z int }

// パーティクル (座標は 1/16px)
type dmPart struct{ x, y, vx, vy, tx, ty int }

var (
	dmStars   [dmNumStars]dmStar
	dmT       = 0
	dmParts   []dmPart
	dmPartBuf [600]dmPart

	dmColStarNear = pixel.NewColor[pixel.RGB565BE](0xFF, 0xFF, 0xFF)
	dmColStarMid  = pixel.NewColor[pixel.RGB565BE](0xA0, 0xA0, 0xB0)
	dmColStarFar  = pixel.NewColor[pixel.RGB565BE](0x50, 0x50, 0x60)
	dmColText     = pixel.NewColor[pixel.RGB565BE](0xFF, 0xE0, 0x40)
)

func dmSpawnStar(s *dmStar, anyZ bool) {
	s.x = int(bkRnd()%960) - 480
	s.y = int(bkRnd()%960) - 480
	if anyZ {
		s.z = 64 + int(bkRnd()%448)
	} else {
		s.z = 512
	}
}

// demoInit はデモ画面に入るときに呼ぶ
func demoInit() {
	dmT = 0
	for i := range dmStars {
		dmSpawnStar(&dmStars[i], true)
	}
	if len(dmParts) == 0 {
		dmBuildText()
	}
}

// dmBuildText は "Go Conference 2026" を pixelBuf に一時描画してインクの
// ピクセルを拾い、パーティクルの目標座標 (2 倍拡大で中央配置) を作る
func dmBuildText() {
	spiBus.Wait() // pixelBuf をスクラッチとして使うため転送完了を待つ

	raw := pixelBuf.RawBuffer()
	for i := range raw {
		raw[i] = 0
	}
	d := &imageDisplayer{img: pixelBuf}
	tinyfont.WriteLine(d, ttFont, 0, 12, "Go Conference 2026", color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})

	n := 0
	const w = 6 * 18 // 半角 18 文字
	for y := 0; y < 16 && n < len(dmPartBuf); y++ {
		for x := 0; x < w && n < len(dmPartBuf); x++ {
			o := (y*240 + x) * 2
			if raw[o] == 0 && raw[o+1] == 0 {
				continue
			}
			dmPartBuf[n].tx = ((240-w*2)/2 + x*2) * 16
			dmPartBuf[n].ty = ((240-16*2)/2 + y*2) * 16
			n++
		}
	}
	dmParts = dmPartBuf[:n]
}

// dmClear は pixelBuf を黒クリアする (倍々コピーで高速化)
func dmClear(raw []uint8) {
	for i := 0; i < 480; i++ {
		raw[i] = 0
	}
	for n := 480; n < len(raw); n *= 2 {
		copy(raw[n:], raw[:n])
	}
}

func dmPlot2(raw []uint8, px, py int, c pixel.RGB565BE) {
	if px < 0 || px > 238 || py < 0 || py > 238 {
		return
	}
	l, h := byte(c), byte(c>>8)
	o := py*480 + px*2
	raw[o], raw[o+1] = l, h
	raw[o+2], raw[o+3] = l, h
	raw[o+480], raw[o+481] = l, h
	raw[o+482], raw[o+483] = l, h
}

func dmPlot1(raw []uint8, px, py int, c pixel.RGB565BE) {
	if px < 0 || px > 239 || py < 0 || py > 239 {
		return
	}
	o := py*480 + px*2
	raw[o], raw[o+1] = byte(c), byte(c>>8)
}

// dmDrawStars はスターフィールドを 1 フレーム進めて描く
func dmDrawStars(raw []uint8) {
	for i := range dmStars {
		s := &dmStars[i]
		s.z -= 4
		if s.z < 48 {
			dmSpawnStar(s, false)
		}
		px := 120 + s.x*128/s.z
		py := 120 + s.y*128/s.z
		if px < -2 || px > 242 || py < -2 || py > 242 {
			dmSpawnStar(s, false)
			continue
		}
		switch {
		case s.z < 160:
			dmPlot2(raw, px, py, dmColStarNear)
		case s.z < 320:
			dmPlot1(raw, px, py, dmColStarMid)
		default:
			dmPlot1(raw, px, py, dmColStarFar)
		}
	}
}

// dmDrawText はパーティクル文字を 1 フレーム進めて描く。t は 0..dmPhaseText-1
func dmDrawText(raw []uint8, t int) {
	if t == 0 {
		// 画面外周のランダムな位置から集合を開始
		for i := range dmParts {
			p := &dmParts[i]
			p.x = (int(bkRnd()%480) - 120) * 16
			p.y = (int(bkRnd()%480) - 120) * 16
		}
	}
	if t == dmTextScatter {
		// 拡散の初速をランダムに与える
		for i := range dmParts {
			p := &dmParts[i]
			p.vx = int(bkRnd()%129) - 64
			p.vy = int(bkRnd()%129) - 64
		}
	}
	for i := range dmParts {
		p := &dmParts[i]
		if t < dmTextScatter {
			// 目標へイーズイン。整数切り捨てで 1px ずれたまま止まらないよう、
			// 近づいたら目標へスナップして綺麗な文字にする
			dx := p.tx - p.x
			dy := p.ty - p.y
			if dx < 8 && dx > -8 && dy < 8 && dy > -8 {
				p.x, p.y = p.tx, p.ty
			} else {
				p.x += dx / 6
				p.y += dy / 6
			}
		} else {
			p.x += p.vx
			p.y += p.vy
		}
		dmPlot2(raw, p.x/16, p.y/16, dmColText)
	}
}

// updateDemo は 30Hz で呼ばれ、1 フレーム描いて全画面を転送する
func updateDemo(display st7789.Device) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	raw := pixelBuf.RawBuffer()
	dmClear(raw)
	dmDrawStars(raw)

	if dmT >= dmPhaseStars1 {
		dmDrawText(raw, dmT-dmPhaseStars1)
	}

	dmT++
	if dmT >= dmPhaseTotal {
		dmT = 0
	}
	return display.DrawBitmap(0, 0, pixelBuf)
}
