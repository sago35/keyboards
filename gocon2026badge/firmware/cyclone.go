package main

import (
	"image/color"
	"machine"
	"strconv"

	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freesans"
)

// サイクロン風「光を止めろ」。16 連 LED リングが主役のタイミングゲーム。
// 白い光がリングを回り続けるので、Go ブルーのターゲットに重なった瞬間に A を
// 押す。ぴったりで PERFECT、±1 で GOOD、それ以外はミス (ライフ -1)。
// 当てるほど回転が速くなる。ターゲットは猶予 (約 2.5 周) 以内に踏むこと。
// 画面にはスコアとリングのミラー表示を出す。U/L でバッジ画面へ戻る。
const (
	cyStartIv  = 8  // 初期回転速度 (フレーム / LED)
	cyMinIv    = 3  // 最高速 (3 フレーム = 100ms / LED)
	cyLives0   = 3  // ライフ
	cyDeadline = 40 // ターゲット設定から許されるスピナー移動数 (= 2.5 周)
	cyFxLen    = 12 // ヒット/ミス演出のフレーム数
)

// 判定
const (
	cyJNone uint8 = iota
	cyJPerfect
	cyJGood
	cyJMiss
)

var (
	cyPos      int // スピナー位置 0..15
	cyIvCount  int // 次の移動までの残りフレーム
	cyInterval int
	cyTarget   int
	cySteps    int // ターゲット設定からのスピナー移動数
	cyScore    int
	cyBest     int
	cyCombo    int
	cyHits     int // 加速管理用の累計ヒット数
	cyLives    int
	cyFxHit    int // ヒット演出の残りフレーム
	cyFxMiss   int // ミス演出の残りフレーム
	cyJudge    = cyJNone
	cyJudgeT   int
	cyOver     bool
	cyOverT    int
	cyPrevA    bool
	cyFrame    int
	cyColors   [16][3]uint8 // 今フレームのリング色 (LED と画面ミラーで共有)
)

var cyBtnA = machine.GPIO3 // 判定はポーリングより細かく毎フレーム直接読む

// 画面ミラー表示のドット座標 (半径 90 の円周 16 分割)。
// 実機に合わせて index 0 が真下、index 増加で反時計回り (下 → 右 → 上 → 左)
var (
	cyDotX = [16]int{0, 34, 64, 83, 90, 83, 64, 34, 0, -34, -64, -83, -90, -83, -64, -34}
	cyDotY = [16]int{90, 83, 64, 34, 0, -34, -64, -83, -90, -83, -64, -34, 0, 34, 64, 83}
)

func cycloneInit() {
	cyPos = 0
	cyInterval = cyStartIv
	cyIvCount = cyInterval
	cyScore = 0
	cyCombo = 0
	cyHits = 0
	cyLives = cyLives0
	cyFxHit, cyFxMiss = 0, 0
	cyJudge = cyJNone
	cyOver = false
	cyFrame = 0
	cyPrevA = true // 入場時の押しっぱなしで誤発火しないように
	cyNewTarget()
}

// cyNewTarget はスピナーの少し先にターゲットを置き直す
func cyNewTarget() {
	cyTarget = (cyPos + 5 + int(bkRnd()%8)) % 16
	cySteps = 0
}

// cyDist は 2 つの LED 位置の円周距離 (0..8)
func cyDist(a, b int) int {
	d := a - b
	if d < 0 {
		d = -d
	}
	if d > 8 {
		d = 16 - d
	}
	return d
}

func cyMissed() {
	cyJudge = cyJMiss
	cyJudgeT = 30
	cyCombo = 0
	cyFxMiss = cyFxLen
	cyLives--
	if cyLives <= 0 {
		cyOver = true
		cyOverT = 0
		return
	}
	cyNewTarget()
}

func cyStep() {
	cyFrame++
	if cyJudgeT > 0 {
		cyJudgeT--
	}

	pressed := !cyBtnA.Get()
	edge := pressed && !cyPrevA
	cyPrevA = pressed

	if cyOver {
		cyOverT++
		if cyOverT > 20 && edge {
			cycloneInit()
		}
		cyComputeColors()
		return
	}

	// ヒット演出中はスピナーを止めて一拍置く
	if cyFxHit > 0 {
		cyFxHit--
		if cyFxHit == 0 {
			// 3 ヒットごとに加速し、次のターゲットへ
			if cyHits%3 == 0 && cyInterval > cyMinIv {
				cyInterval--
			}
			cyNewTarget()
		}
		cyComputeColors()
		return
	}
	if cyFxMiss > 0 {
		cyFxMiss--
	}

	// スピナーの回転
	cyIvCount--
	if cyIvCount <= 0 {
		cyIvCount = cyInterval
		cyPos = (cyPos + 1) % 16
		cySteps++
		if cySteps > cyDeadline { // 猶予切れ
			cyMissed()
			cyComputeColors()
			return
		}
	}

	// 判定
	if edge {
		switch cyDist(cyPos, cyTarget) {
		case 0:
			cyJudge = cyJPerfect
			cyJudgeT = 30
			cyCombo++
			cyHits++
			cyScore += 100 + cyCombo*10
			cyFxHit = cyFxLen
		case 1:
			cyJudge = cyJGood
			cyJudgeT = 30
			cyCombo++
			cyHits++
			cyScore += 50 + cyCombo*10
			cyFxHit = cyFxLen
		default:
			cyMissed()
		}
		if cyScore > cyBest {
			cyBest = cyScore
		}
	}

	cyComputeColors()
}

// cyComputeColors はリング 16 個ぶんの色を決める (LED と画面ミラーで共有)
func cyComputeColors() {
	// ベース (完全消灯にせず、うっすら点けておく)
	for i := range cyColors {
		cyColors[i] = [3]uint8{0x20, 0x20, 0x30}
	}

	if cyOver {
		// ゲームオーバー: 赤の呼吸
		v := uint8(0x30 + yOffsets[(cyFrame/2)%len(yOffsets)]*4)
		for i := range cyColors {
			cyColors[i] = [3]uint8{v, 0x04, 0x04}
		}
		return
	}

	if cyFxHit > 0 {
		// ヒット: ターゲットから広がる虹のリップル
		age := cyFxLen - cyFxHit
		for i := range cyColors {
			h := (cyDist(i, cyTarget)*40 + age*36) % 360
			r, g, b := hsvToRGB(h, 255, 200)
			cyColors[i] = [3]uint8{r, g, b}
		}
		return
	}

	if cyFxMiss > 0 {
		// ミス: 全体が赤くフラッシュして減衰
		v := uint8(cyFxMiss * 255 / cyFxLen)
		for i := range cyColors {
			cyColors[i] = [3]uint8{v, 0, 0}
		}
		return
	}

	// スピナーの尾 (ベースの明るさへ滑らかにつなげる)
	trail := [4]uint8{0x80, 0x50, 0x38, 0x28}
	for t := 1; t <= 4; t++ {
		v := trail[t-1]
		cyColors[(cyPos-t+16)%16] = [3]uint8{v, v, v + 0x08}
	}
	// ターゲット (猶予が残り少ないと点滅)
	show := true
	if cySteps > cyDeadline-16 && (cyFrame/3)&1 == 0 {
		show = false
	}
	if show {
		cyColors[cyTarget] = [3]uint8{0x00, 0xAD, 0xD8}
	}
	// スピナー本体 (ターゲット上では白 + 青のまぜ色)
	if cyPos == cyTarget {
		cyColors[cyPos] = [3]uint8{0xA0, 0xFF, 0xFF}
	} else {
		cyColors[cyPos] = [3]uint8{0xFF, 0xFF, 0xFF}
	}
}

func cyFill(raw []uint8, x0, y0, w, h int, c pixel.RGB565BE) {
	if x0 < 0 {
		w += x0
		x0 = 0
	}
	if y0 < 0 {
		h += y0
		y0 = 0
	}
	if x0+w > 240 {
		w = 240 - x0
	}
	if y0+h > 240 {
		h = 240 - y0
	}
	if w <= 0 || h <= 0 {
		return
	}
	l, hh := byte(c), byte(c>>8)
	base := y0*480 + x0*2
	for x := 0; x < w; x++ {
		raw[base+x*2], raw[base+x*2+1] = l, hh
	}
	for y := 1; y < h; y++ {
		copy(raw[base+y*480:base+y*480+w*2], raw[base:base+w*2])
	}
}

func cyRender(raw []uint8) {
	// 背景
	bg := pixel.NewColor[pixel.RGB565BE](0x04, 0x06, 0x10)
	cyFill(raw, 0, 0, 240, 240, bg)

	// リングのミラー表示 (LED と同じ色)
	for i := 0; i < 16; i++ {
		c := cyColors[i]
		col := pixel.NewColor[pixel.RGB565BE](c[0], c[1], c[2])
		size := 10
		if i == cyPos || i == cyTarget {
			size = 14
		}
		cyFill(raw, 120+cyDotX[i]-size/2, 120+cyDotY[i]-size/2, size, size, col)
	}

	d := &imageDisplayer{img: pixelBuf}
	white := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	// スコア (中央に大きく)
	s := strconv.Itoa(cyScore)
	_, sw := tinyfont.LineWidth(&freesans.Bold24pt7b, s)
	tinyfont.WriteLine(d, &freesans.Bold24pt7b, int16(120-int(sw)/2), 132, s, white)

	// 判定表示
	if cyJudgeT > 0 {
		switch cyJudge {
		case cyJPerfect:
			tinyfont.WriteLine(d, ttFont, 96, 165, "PERFECT!", color.RGBA{R: 0xFF, G: 0xD8, B: 0x30, A: 0xFF})
		case cyJGood:
			tinyfont.WriteLine(d, ttFont, 105, 165, "GOOD", color.RGBA{R: 0x00, G: 0xAD, B: 0xD8, A: 0xFF})
		case cyJMiss:
			tinyfont.WriteLine(d, ttFont, 105, 165, "MISS", color.RGBA{R: 0xFF, G: 0x40, B: 0x40, A: 0xFF})
		}
	}
	// コンボ
	if cyCombo >= 2 {
		cs := strconv.Itoa(cyCombo) + " COMBO"
		tinyfont.WriteLine(d, ttFont, int16(120-len(cs)*3), 90, cs, white)
	}

	// ライフ (左上) と HI (右上)
	for i := 0; i < cyLives; i++ {
		cyFill(raw, 8+i*12, 8, 8, 8, pixel.NewColor[pixel.RGB565BE](0xE0, 0x40, 0x60))
	}
	hi := "HI:" + strconv.Itoa(cyBest)
	tinyfont.WriteLine(d, ttFont, int16(232-len(hi)*6), 15, hi, white)

	if cyOver {
		cyFill(raw, 30, 180, 180, 40, bg)
		tinyfont.WriteLine(d, ttFont, 93, 196, "GAME OVER", white)
		tinyfont.WriteLine(d, ttFont, 81, 212, "A:リトライ", white)
	}
}

// updateCyclone は 30Hz で呼ばれる
func updateCyclone(display st7789.Device) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	cyStep()
	cyRender(pixelBuf.RawBuffer())
	return display.DrawBitmap(0, 0, pixelBuf)
}
