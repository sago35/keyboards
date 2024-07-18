package main

import (
	_ "embed"
	"image/color"
	"log"
	"machine"
	"machine/usb"
	"machine/usb/hid/mouse"
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

var (
	white = color.RGBA{0x3F, 0x3F, 0x3F, 0xFF}
	red   = color.RGBA{0xFF, 0x00, 0x00, 0xFF}
	green = color.RGBA{0x00, 0xFF, 0x00, 0xFF}
	blue  = color.RGBA{0x00, 0x00, 0xFF, 0xFF}
	black = color.RGBA{0x00, 0x00, 0x00, 0xFF}
)

func writeColors(s pio.StateMachine, ws *piolib.WS2812, colors []color.RGBA) {
	for _, c := range colors {
		for s.IsTxFIFOFull() {
		}
		ws.SetColor(c)
	}
}

func run() error {
	i2c := machine.I2C0
	i2c.Configure(machine.I2CConfig{
		Frequency: machine.TWI_FREQ_400KHZ,
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
	ws, _ := piolib.NewWS2812(s, wsPin)
	wsLeds := [12]color.RGBA{}
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

	d.AddMatrixKeyboard(colPins, rowPins, [][]keyboard.Keycode{
		{
			jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD,
			jp.KeyE, jp.KeyF, jp.KeyG, jp.KeyH,
			jp.KeyI, jp.KeyJ, jp.KeyK, jp.KeyL,
		},
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
	})
	rkIndex := 0
	rk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == 2 {
			if index == 0 {
				rkIndex = (rkIndex + 1) % 10
				wsLeds[4] = red
			} else {
				rkIndex = (rkIndex - 1 + 10) % 10
				wsLeds[7] = blue
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
			wsLeds[idx] = green
			interrupt.Restore(mask)
			changed.Set(1)
		}
	})

	gpioPins := []machine.Pin{machine.GPIO0, machine.GPIO2}
	for c := range gpioPins {
		gpioPins[c].Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	d.AddGpioKeyboard(gpioPins, [][]keyboard.Keycode{
		{
			jp.MouseLeft, jp.MouseRight,
		},
	})

	loadKeyboardDef()

	d.Init()
	cont := true
	x := NewADCDevice(ax, 0x3000, 0xC800, false)
	y := NewADCDevice(ay, 0x3000, 0xC800, true)
	ticker := time.Tick(1 * time.Millisecond)
	cnt := 0

	dispx := int16(0)
	dispy := int16(0)
	deltaX := int16(1)
	deltaY := int16(1)
	for cont {
		<-ticker
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
				c.B >>= 1
				c.R >>= 1
				c.G >>= 1
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
