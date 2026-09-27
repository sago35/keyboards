package main

import (
	"image/color"

	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freesans"
)

const (
	marqueeText   = "Go Conference 2026"
	marqueeHeight = 56
	marqueeY      = 240 - marqueeHeight // 画面下端の帯 (gopher の可動域 y<168 と重ならないこと)
	marqueeSpeed  = 4                   // 1 回の更新で進むピクセル数
)

var (
	marqueeFont  = &freesans.Bold24pt7b
	marqueeX     = 240 // 右端の画面外からスクロールイン
	marqueeWidth int
	marqueeColor = color.RGBA{R: 0x00, G: 0xAD, B: 0xD8, A: 0xFF} // Go ブランドカラー
)

func init() {
	_, w := tinyfont.LineWidth(marqueeFont, marqueeText)
	marqueeWidth = int(w)
}

// imageDisplayer は tinyfont から pixel.Image へ描き込むためのアダプタ。
// 範囲外の描画は無視するので、はみ出す文字のクリッピングを兼ねる。
type imageDisplayer struct {
	img pixel.Image[pixel.RGB565BE]
}

func (d *imageDisplayer) Size() (int16, int16) {
	w, h := d.img.Size()
	return int16(w), int16(h)
}

func (d *imageDisplayer) SetPixel(x, y int16, c color.RGBA) {
	w, h := d.img.Size()
	if x < 0 || y < 0 || int(x) >= w || int(y) >= h {
		return
	}
	d.img.Set(int(x), int(y), pixel.NewColor[pixel.RGB565BE](c.R, c.G, c.B))
}

func (d *imageDisplayer) Display() error { return nil }

// composeMarquee は pixelBuf の帯領域を背景に戻してから現在位置に文字を描く。
func composeMarquee() {
	raw := pixelBuf.RawBuffer()
	copy(raw[marqueeY*240*2:], background565[marqueeY*240*2:])
	d := &imageDisplayer{img: pixelBuf}
	// ベースラインは帯の下端から 12px 上 (24pt のディセンダ ~10px が収まる位置)
	tinyfont.WriteLine(d, marqueeFont, int16(marqueeX), marqueeY+marqueeHeight-12, marqueeText, marqueeColor)
}

// drawMarquee はスクロールを 1 ステップ進め、帯領域だけを転送する。
func drawMarquee(display st7789.Device) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	marqueeX -= marqueeSpeed
	if marqueeX < -marqueeWidth {
		marqueeX = 240
	}
	composeMarquee()

	raw := pixelBuf.RawBuffer()
	band := pixel.NewImageFromBytes[pixel.RGB565BE](240, marqueeHeight, raw[marqueeY*240*2:])
	return display.DrawBitmap(0, marqueeY, band)
}
