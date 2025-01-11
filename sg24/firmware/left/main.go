package main

import (
	"context"
	_ "embed"
	"fmt"
	"image/color"
	"log"
	"machine"
	"machine/usb"

	keyboard "github.com/sago35/tinygo-keyboard"
	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
	"tinygo.org/x/drivers/ws2812"
)

func main() {
	usb.Product = "sg24-0.1.0"

	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

var (
	red   = []color.RGBA{{R: 0xFF, G: 0x00, B: 0x00}}
	blue  = []color.RGBA{{R: 0x00, G: 0x00, B: 0xFF}}
	white = []color.RGBA{{R: 0x33, G: 0x33, B: 0x33}}
)

func writeColors(ws ws2812.Device, c int) {
	switch c {
	case 1:
		ws.WriteColors(blue)
	case 2:
		ws.WriteColors(red)
	default:
		ws.WriteColors(white)
	}
}

func run() error {
	neo := machine.NEOPIXEL
	neo.Configure(machine.PinConfig{Mode: machine.PinOutput})
	ws := ws2812.New(neo)
	ws.WriteColors(white)

	d := keyboard.New()

	colPins := []machine.Pin{
		machine.D2,
		machine.D3,
		machine.D4,
		machine.D5,
		machine.D6,
		machine.D7,
	}

	sm := d.AddSquaredMatrixKeyboard(colPins, [][]keyboard.Keycode{
		{
			jp.KeyTab, jp.KeyQ, jp.KeyW, jp.KeyE, jp.KeyR, jp.KeyT,
			jp.KeyLeftCtrl, jp.KeyA, jp.KeyS, jp.KeyD, jp.KeyF, jp.KeyG,
			jp.KeyLeftShift, jp.KeyZ, jp.KeyX, jp.KeyC, jp.KeyV, jp.KeyB,
			jp.KeyEsc, jp.KeyWindows, jp.KeyLeftAlt, jp.KeyMod1, jp.KeySpace,
		},
		{
			jp.KeyTab, jp.KeyQ, jp.KeyF15, jp.KeyEnd, jp.KeyF17, jp.KeyF18,
			jp.KeyLeftCtrl, jp.KeyHome, jp.KeyS, jp.MouseRight, jp.MouseLeft, jp.MouseBack,
			jp.KeyLeftShift, jp.KeyF13, jp.KeyF14, jp.MouseMiddle, jp.KeyF16, jp.MouseForward,
			jp.KeyEsc, jp.KeyWindows, jp.KeyLeftAlt, jp.KeyMod1, jp.KeySpace, jp.KeySpace,
		},
		{
			jp.KeyTab, jp.Key1, jp.Key2, jp.Key3, jp.Key4, jp.Key5,
			jp.KeyLeftCtrl, jp.KeyMinus, jp.KeyHat, jp.KeyBackslash2, jp.KeyLeftBrace, jp.KeyRightBrace,
			jp.KeyLeftShift, jp.KeyF1, jp.KeyF2, jp.KeyF3, jp.KeyF4, jp.KeyF5,
			jp.KeyEsc, jp.KeyWindows, jp.KeyLeftAlt, jp.KeyMod1, jp.KeySpace, jp.KeySpace,
		},
	})
	sm.SetCallback(func(layer, index int, state keyboard.State) {
		layer = d.Layer()
		fmt.Printf("sm: %d %d %d\n", layer, index, state)
		writeColors(ws, layer)
	})

	uart := machine.UART0
	uart.Configure(machine.UARTConfig{TX: machine.NoPin, RX: machine.UART0_RX_PIN})

	uk := d.AddUartKeyboard(24, uart, [][]keyboard.Keycode{
		{
			jp.KeyY, jp.KeyU, jp.KeyI, jp.KeyO, jp.KeyP, jp.KeyAt,
			jp.KeyH, jp.KeyJ, jp.KeyK, jp.KeyL, jp.KeySemicolon, jp.KeyColon,
			jp.KeyN, jp.KeyM, jp.KeyComma, jp.KeyPeriod, jp.KeySlash, jp.KeyBackslash,
			jp.KeySpace, jp.KeyMod2, jp.KeyHiragana, jp.KeyLeftAlt, jp.KeyPrintscreen, jp.KeyDelete,
		},
		{
			jp.KeyY, jp.KeyU, jp.KeyTab, jp.KeyO, jp.WheelUp, jp.KeyAt,
			jp.KeyLeft, jp.KeyDown, jp.KeyUp, jp.KeyRight, jp.KeyEnter, jp.KeyEsc,
			jp.WheelDown, jp.KeyM, jp.KeyComma, jp.KeyPeriod, jp.KeySlash, jp.KeyBackslash,
			jp.KeySpace, jp.KeyMod2, jp.KeyHiragana, jp.KeyTo2, jp.KeyPrintscreen, jp.KeyDelete,
		},
		{
			jp.Key6, jp.Key7, jp.Key8, jp.Key9, jp.Key0, jp.KeyBackspace,
			jp.KeyHome, jp.KeyPageDown, jp.KeyPageUp, jp.KeyEnd, jp.KeyEnter, jp.KeyEsc,
			jp.KeyF6, jp.KeyF7, jp.KeyF8, jp.KeyF9, jp.KeyF10, jp.KeyF11,
			jp.KeySpace, jp.KeyMod2, jp.KeyHiragana, jp.KeyTo0, jp.KeyPrintscreen, jp.KeyF12,
		},
	})
	uk.SetCallback(func(layer, index int, state keyboard.State) {
		layer = d.Layer()
		fmt.Printf("uk: %d %d %d\n", layer, index, state)
		writeColors(ws, layer)
	})

	// override ctrl-h to BackSpace
	d.OverrideCtrlH()

	loadKeyboardDef()

	return d.Loop(context.Background())
}
