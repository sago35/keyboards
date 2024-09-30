package main

import (
	"context"
	_ "embed"
	"log"
	"machine"
	"machine/usb"

	keyboard "github.com/sago35/tinygo-keyboard"
	"github.com/sago35/tinygo-keyboard/keycodes/jp"
	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
)

var (
	invertRotaryPins = false
)

func main() {
	usb.Product = "panel25-0.1.0"

	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

const (
	white = 0x3F3F3FFF
	red   = 0x00FF00FF
	green = 0xFF0000FF
	blue  = 0x0000FFFF
	black = 0x000000FF
)

func writeColors(s pio.StateMachine, ws *piolib.WS2812B, colors []uint32) {
	ws.WriteRaw(colors)
}

func run() error {
	wsPin := machine.GPIO26
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, wsPin)
	err := ws.EnableDMA(true)
	if err != nil {
		return err
	}
	wsLeds := [100]uint32{}
	for i := range wsLeds {
		wsLeds[i] = red
	}
	writeColors(s, ws, wsLeds[:])

	colPins := []machine.Pin{
		machine.GPIO0,
		machine.GPIO1,
		machine.GPIO2,
		machine.GPIO3,
		machine.GPIO4,
	}

	rowPins := []machine.Pin{
		machine.GPIO5,
		machine.GPIO6,
		machine.GPIO7,
		machine.GPIO8,
		machine.GPIO9,
		machine.GPIO10,
		machine.GPIO11,
		machine.GPIO27,
		machine.GPIO28,
		machine.GPIO29,
	}

	d := keyboard.New()

	d.AddDuplexMatrixKeyboard(colPins, rowPins, [][]keyboard.Keycode{
		{
			jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD, jp.KeyE, jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD, jp.KeyE,
			jp.KeyF, jp.KeyG, jp.KeyH, jp.KeyI, jp.KeyJ, jp.KeyF, jp.KeyG, jp.KeyH, jp.KeyI, jp.KeyJ,
			jp.KeyK, jp.KeyL, jp.KeyM, jp.KeyN, jp.KeyO, jp.KeyK, jp.KeyL, jp.KeyM, jp.KeyN, jp.KeyO,
			jp.KeyP, jp.KeyQ, jp.KeyR, jp.KeyS, jp.KeyT, jp.KeyP, jp.KeyQ, jp.KeyR, jp.KeyS, jp.KeyT,
			jp.KeyU, jp.KeyV, jp.KeyW, jp.KeyX, jp.KeyY, jp.KeyU, jp.KeyV, jp.KeyW, jp.KeyX, jp.KeyY,

			jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD, jp.KeyE, jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD, jp.KeyE,
			jp.KeyF, jp.KeyG, jp.KeyH, jp.KeyI, jp.KeyJ, jp.KeyF, jp.KeyG, jp.KeyH, jp.KeyI, jp.KeyJ,
			jp.KeyK, jp.KeyL, jp.KeyM, jp.KeyN, jp.KeyO, jp.KeyK, jp.KeyL, jp.KeyM, jp.KeyN, jp.KeyO,
			jp.KeyP, jp.KeyQ, jp.KeyR, jp.KeyS, jp.KeyT, jp.KeyP, jp.KeyQ, jp.KeyR, jp.KeyS, jp.KeyT,
			jp.KeyU, jp.KeyV, jp.KeyW, jp.KeyX, jp.KeyY, jp.KeyU, jp.KeyV, jp.KeyW, jp.KeyX, jp.KeyY,
		},
	})

	loadKeyboardDef()

	return d.Loop(context.Background())
}
