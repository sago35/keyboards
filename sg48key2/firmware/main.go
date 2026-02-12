package main

import (
	_ "embed"
	"fmt"
	"log"
	"machine"
	"machine/usb"
	"machine/usb/hid/mouse"
	"math/rand/v2"
	"time"

	keyboard "github.com/sago35/tinygo-keyboard"
	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
)

func main() {
	usb.Product = "sg48key2-0.1.0"

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
	machine.InitADC()
	ax := machine.GPIO27
	ay := machine.GPIO26

	m := mouse.Port()

	wsPin := machine.GPIO14
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, wsPin)
	err := ws.EnableDMA(true)
	if err != nil {
		return err
	}
	wsLeds := [48]uint32{}
	for i := range wsLeds {
		wsLeds[i] = black
	}
	writeColors(s, ws, wsLeds[:])

	d := keyboard.New()

	colPins := []machine.Pin{
		machine.D0,
		machine.D1,
		machine.D2,
		machine.D3,
		machine.D4,
		machine.D5,
		machine.D6,
		machine.D7, // not connected
	}

	sm := d.AddSquaredMatrixKeyboard(colPins, [][]keyboard.Keycode{
		{
			jp.KeyTab, jp.KeyQ, jp.KeyW, jp.KeyE, jp.KeyR, jp.KeyT, jp.KeyY, jp.KeyU, jp.KeyI, jp.KeyO, jp.KeyP, jp.KeyAt,
			jp.KeyLeftCtrl, jp.KeyA, jp.KeyS, jp.KeyD, jp.KeyF, jp.KeyG, jp.KeyH, jp.KeyJ, jp.KeyK, jp.KeyL, jp.KeySemicolon, jp.KeyColon,
			jp.KeyLeftShift, jp.KeyZ, jp.KeyX, jp.KeyC, jp.KeyV, jp.KeyB, jp.KeyN, jp.KeyM, jp.KeyComma, jp.KeyPeriod, jp.KeySlash, jp.KeyBackslash,
			jp.KeyEsc, jp.KeyWindows, jp.KeyLeftAlt, jp.KeyMod1, jp.KeySpace, jp.KeySpace, jp.KeySpace, jp.KeyMod2, jp.KeyHiragana, 0, jp.KeyPrintscreen, jp.KeyDelete,
		},
		{
			jp.KeyTab, jp.KeyQ, jp.KeyF15, jp.KeyEnd, jp.KeyF17, jp.KeyF18, jp.KeyY, jp.KeyU, jp.KeyTab, jp.KeyO, jp.WheelUp, jp.KeyAt,
			jp.KeyLeftCtrl, jp.KeyHome, jp.KeyS, jp.MouseRight, jp.MouseLeft, jp.MouseBack, jp.KeyLeft, jp.KeyDown, jp.KeyUp, jp.KeyRight, jp.KeyEnter, jp.KeyEsc,
			jp.KeyLeftShift, jp.KeyF13, jp.KeyF14, jp.MouseMiddle, jp.KeyF16, jp.MouseForward, jp.WheelDown, jp.KeyM, jp.KeyComma, jp.KeyPeriod, jp.KeySlash, jp.KeyBackslash,
			jp.KeyEsc, jp.KeyWindows, jp.KeyLeftAlt, jp.KeyMod1, jp.KeySpace, jp.KeySpace, jp.KeySpace, jp.KeyMod2, jp.KeyHiragana, jp.KeyTo2, jp.KeyPrintscreen, jp.KeyDelete,
		},
		{
			jp.KeyTab, jp.Key1, jp.Key2, jp.Key3, jp.Key4, jp.Key5, jp.Key6, jp.Key7, jp.Key8, jp.Key9, jp.Key0, jp.KeyBackspace,
			jp.KeyLeftCtrl, jp.KeyMinus, jp.KeyHat, jp.KeyBackslash2, jp.KeyLeftBrace, jp.KeyRightBrace, jp.KeyHome, jp.KeyPageDown, jp.KeyPageUp, jp.KeyEnd, jp.KeyEnter, jp.KeyEsc,
			jp.KeyLeftShift, jp.KeyF1, jp.KeyF2, jp.KeyF3, jp.KeyF4, jp.KeyF5, jp.KeyF6, jp.KeyF7, jp.KeyF8, jp.KeyF9, jp.KeyF10, jp.KeyF11,
			jp.KeyEsc, jp.KeyWindows, jp.KeyLeftAlt, jp.KeyMod1, jp.KeySpace, jp.KeySpace, jp.KeySpace, jp.KeyMod2, jp.KeyHiragana, jp.KeyTo0, jp.KeyPrintscreen, jp.KeyF12,
		},
	})
	sm.SetCallback(func(layer, index int, state keyboard.State) {
		layer = d.Layer()
		fmt.Printf("sm: %d %d %d\n", layer, index, state)
		if state == keyboard.PressToRelease {
			return
		}
		wsLeds[index] = rand.Uint32()
	})

	// override ctrl-h to BackSpace
	d.OverrideCtrlH()

	// Combos
	combos := []keyboard.Combo{
		{
			Keys:      [4]keyboard.Keycode{jp.KeyQ, jp.KeyZ},
			OutputKey: jp.KeyMediaMute,
		},
		{
			Keys:      [4]keyboard.Keycode{jp.KeyW, jp.KeyX},
			OutputKey: jp.KeyMediaVolumeDec,
		},
		{
			Keys:      [4]keyboard.Keycode{jp.KeyE, jp.KeyC},
			OutputKey: jp.KeyMediaVolumeInc,
		},
		{
			Keys:      [4]keyboard.Keycode{jp.KeyR, jp.KeyV},
			OutputKey: jp.KeyMediaBrightnessDown,
		},
		{
			Keys:      [4]keyboard.Keycode{jp.KeyT, jp.KeyB},
			OutputKey: jp.KeyMediaBrightnessUp,
		},
	}
	for i, c := range combos {
		d.SetCombo(i, c)
	}

	loadKeyboardDef()

	err = d.Init()
	if err != nil {
		return err
	}

	cont := true
	x := NewADCDevice(ax, 0x1000, 0xF000, false)
	y := NewADCDevice(ay, 0x1000, 0xF000, false)
	ticker := time.Tick(500 * time.Microsecond)
	cnt := 0
	for cont {
		<-ticker
		err := d.Tick()
		if err != nil {
			return err
		}

		switch cnt % (5 * 3) {
		case 0:
			xx := x.Get2()
			yy := y.Get2()
			if !(xx == 0 && yy == 0) {
				//fmt.Printf("%04X %04X %4d %4d %4d %4d\n", x.RawValue, y.RawValue, xx, yy, x.Get(), y.Get())
				m.Move(int(xx), int(yy))
			}
		case 10:
			writeColors(s, ws, wsLeds[:])
			for i, c := range wsLeds {
				g := (c & 0xFF000000) >> 24
				r := (c & 0x00FF0000) >> 16
				b := (c & 0x0000FF00) >> 8
				const xv = 1
				if g > xv {
					g -= xv
				} else {
					g = 0
				}
				if r > xv {
					r -= xv
				} else {
					r = 0
				}
				if b > xv {
					b -= xv
				} else {
					b = 0
				}
				c = g<<24 | r<<16 | b<<8 | 0xFF
				wsLeds[i] = c
			}
		}
		cnt++
	}

	return nil
}
