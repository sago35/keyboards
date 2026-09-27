package main

import (
	_ "embed"

	"tinygo.org/x/drivers/st7789"
)

// 名札画面と QR コード画面。画像は PC 側でレンダリングした RGB565(BE) raw を
// 焼き込む (生成ツールと元 PNG は scratchpad の gen-nametag / firmware 直下の
// nametag.png, qrcode.png を参照)。

//go:embed images/nametag.rgb565
var nametagImg string

//go:embed images/qrcode.rgb565
var qrcodeImg string

// drawFullImage は 240x240 の RGB565 raw を全画面に表示する
func drawFullImage(display st7789.Device, img string) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	copy(pixelBuf.RawBuffer(), img)
	return display.DrawBitmap(0, 0, pixelBuf)
}
