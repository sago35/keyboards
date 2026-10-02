package main

// WS2812 の配線確認用テスト。BLE を使わず全 LED を白で 500ms 点滅させる。
// main.go の ledTest を true にすると有効になる。

import (
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/drivers/ws2812"
)

func runLedTest() {
	wsPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	ws := ws2812.NewWS2812(wsPin)

	colors := make([]color.RGBA, 100)
	for {
		for i := range colors {
			colors[i] = color.RGBA{}
		}
		ws.WriteColors(colors)
		time.Sleep(500 * time.Millisecond)
		for i := 0; i < 100; i++ {
			colors[i] = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF}
		}
		ws.WriteColors(colors)
		time.Sleep(500 * time.Millisecond)
	}
}
