package main

import (
	_ "embed"
	"image/color"
	"log"
	"machine"
	"machine/usb"
	"machine/usb/hid/mouse"
	"math/rand/v2"
	"runtime/interrupt"
	"runtime/volatile"
	"time"

	keyboard "github.com/sago35/tinygo-keyboard"
	"github.com/sago35/tinygo-keyboard/keycodes/jp"
	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
	"tinygo.org/x/drivers/ssd1306"
)

var (
	invertRotaryPins = false
)

func main() {
	usb.Product = "zero-kb02-0.1.0"

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
	i2c := machine.I2C0
	i2c.Configure(machine.I2CConfig{
		Frequency: 2.8 * machine.MHz,
		SDA:       machine.GPIO12,
		SCL:       machine.GPIO13,
	})

	display := ssd1306.NewI2C(i2c)
	display.Configure(ssd1306.Config{
		Address: 0x3C,
		Width:   128,
		Height:  64,
	})
	display.ClearDisplay()

	var changed volatile.Register8
	changed.Set(0)

	wsPin := machine.GPIO1
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, wsPin)
	err := ws.EnableDMA(true)
	if err != nil {
		return err
	}
	wsLeds := [12]uint32{}
	for i := range wsLeds {
		wsLeds[i] = black
	}
	writeColors(s, ws, wsLeds[:])

	machine.InitADC()
	ax := machine.GPIO29
	ay := machine.GPIO28

	m := mouse.Port()

	d := keyboard.New()

	colPins := []machine.Pin{
		machine.GPIO5,
		machine.GPIO6,
		machine.GPIO7,
		machine.GPIO8,
	}

	rowPins := []machine.Pin{
		machine.GPIO9,
		machine.GPIO10,
		machine.GPIO11,
	}

	mk := d.AddMatrixKeyboard(colPins, rowPins, [][]keyboard.Keycode{
		{
			jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD,
			jp.KeyE, jp.KeyF, jp.KeyG, jp.KeyH,
			jp.KeyMod1, jp.KeyMod2, jp.MouseLeft, jp.MouseRight,
		},
		{
			jp.KeyI, jp.KeyJ, jp.KeyK, jp.KeyL,
			jp.KeyM, jp.KeyN, jp.KeyO, jp.KeyP,
			jp.KeyMod1, jp.KeyMod2, jp.MouseLeft, jp.MouseRight,
		},
		{
			jp.KeyQ, jp.KeyR, jp.KeyS, jp.KeyT,
			jp.KeyU, jp.KeyV, jp.KeyW, jp.KeyX,
			jp.KeyMod1, jp.KeyMod2, jp.KeyY, jp.KeyZ,
		},
	})
	mk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == keyboard.PressToRelease {
			return
		}
		mask := interrupt.Disable()
		idx := 0
		switch index {
		case 0:
			idx = 0
		case 1:
			idx = 3
		case 2:
			idx = 6
		case 3:
			idx = 9
		case 4:
			idx = 1
		case 5:
			idx = 4
		case 6:
			idx = 7
		case 7:
			idx = 10
		case 8:
			idx = 2
		case 9:
			idx = 5
		case 10:
			idx = 8
		case 11:
			idx = 11
		}
		wsLeds[idx] = rand.Uint32()
		interrupt.Restore(mask)
		changed.Set(1)
	})

	rotaryPins := []machine.Pin{
		machine.GPIO3,
		machine.GPIO4,
	}
	if invertRotaryPins {
		rotaryPins[0], rotaryPins[1] = rotaryPins[1], rotaryPins[0]
	}

	rk := d.AddRotaryKeyboard(rotaryPins[0], rotaryPins[1], [][]keyboard.Keycode{
		{
			jp.KeyMediaVolumeDec, jp.KeyMediaVolumeInc,
		},
		{
			jp.KeyLeft, jp.KeyRight,
		},
		{
			jp.KeyMediaBrightnessDown, jp.KeyMediaBrightnessUp,
		},
	})
	rkIndex := 0
	rk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == keyboard.Press {
			if index == 0 {
				rkIndex = (rkIndex + 1) % 10
			} else {
				rkIndex = (rkIndex - 1 + 10) % 10
			}
			idx := rkIndex
			switch rkIndex {
			case 0:
				idx = 0
			case 1:
				idx = 1
			case 2:
				idx = 2
			case 3:
				idx = 5
			case 4:
				idx = 8
			case 5:
				idx = 11
			case 6:
				idx = 10
			case 7:
				idx = 9
			case 8:
				idx = 6
			case 9:
				idx = 3
			}
			mask := interrupt.Disable()
			wsLeds[idx] = rand.Uint32()
			interrupt.Restore(mask)
			changed.Set(1)
		}
	})

	gpioPins := []machine.Pin{machine.GPIO0, machine.GPIO2}
	for c := range gpioPins {
		gpioPins[c].Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	gk := d.AddGpioKeyboard(gpioPins, [][]keyboard.Keycode{
		{
			jp.MouseLeft, jp.KeyTo1,
		},
		{
			jp.MouseLeft, jp.KeyTo2,
		},
		{
			jp.MouseLeft, jp.KeyTo0,
		},
	})
	gk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == keyboard.PressToRelease {
			return
		}
		mask := interrupt.Disable()
		idx := 4
		if index == 1 {
			idx = 7
		}
		wsLeds[idx] = rand.Uint32()
		interrupt.Restore(mask)
		changed.Set(1)
	})

	loadKeyboardDef()

	d.Init()
	cont := true
	x := NewADCDevice(ax, 0x3000, 0xC800, false)
	y := NewADCDevice(ay, 0x3000, 0xC800, true)
	cnt := 0

	dispx := int16(0)
	dispy := int16(0)
	deltaX := int16(1)
	deltaY := int16(1)
	for cont {
		time.Sleep(1 * time.Millisecond)
		err := d.Tick()
		if err != nil {
			return err
		}

		if cnt%10 == 0 {
			xx := x.Get2()
			yy := y.Get2()
			//fmt.Printf("%04X %04X %4d %4d %4d %4d\n", x.RawValue, y.RawValue, xx, yy, x.Get(), y.Get())
			m.Move(int(xx), int(yy))
		}

		if cnt%32 == 0 {
			mask := interrupt.Disable()
			for i, c := range wsLeds {
				g := ((c & 0xFF000000) >> 1) & 0xFF000000
				r := ((c & 0x00FF0000) >> 1) & 0x00FF0000
				b := ((c & 0x0000FF00) >> 1) & 0x0000FF00
				c = g | r | b | 0xFF
				wsLeds[i] = c
			}
			writeColors(s, ws, wsLeds[:])
			interrupt.Restore(mask)
		}

		if cnt%32 == 16 {
			pixel := display.GetPixel(dispx, dispy)
			c := color.RGBA{255, 255, 255, 255}
			if pixel {
				c = color.RGBA{0, 0, 0, 255}
			}
			display.SetPixel(dispx, dispy, c)
			display.Display()

			dispx += deltaX
			dispy += deltaY

			if dispx == 0 || dispx == 127 {
				deltaX = -deltaX
			}

			if dispy == 0 || dispy == 63 {
				deltaY = -deltaY
			}
		}

		cnt++
	}

	return nil
}
