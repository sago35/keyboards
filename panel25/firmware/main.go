package main

import (
	_ "embed"
	"image/color"
	"log"
	"machine"
	"machine/usb"
	"runtime"
	"time"

	"github.com/mattn/go-runewidth"
	keyboard "github.com/sago35/tinygo-keyboard"
	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/shnm"
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

	err = d.Init()
	if err != nil {
		return err
	}

	cont := true
	cnt := int16(0)
	display := NewSK6812()
	time.Sleep(2 * time.Second)
	str := "TinyGo Keeb Tour 2025 in Osaka"

	ticker := time.Tick(1 * time.Millisecond)
	cnt2 := 0
	for cont {
		<-ticker
		err := d.Tick()
		if err != nil {
			return err
		}

		if cnt2%100 == 0 {
			for i := int16(0); i < 100; i++ {
				display.SetPixel(i%10, i/10, color.RGBA{R: 0x00, G: 0x00, B: 0x00})
			}

			tinyfont.WriteLine(display, &shnm.Shnmk12, 10+cnt*-1, 9, str, color.RGBA{R: 0x00, G: 0xFF, B: 0x00})

			writeColors(s, ws, display.RawColors())
			cnt = (cnt + 1) % (int16(runewidth.StringWidth(str))*7 + 8)
		}
		cnt2++

		runtime.Gosched()
	}

	return nil
}
