package main

import (
	_ "embed"
	"errors"
	"image/color"
	"log"
	"machine"
	"machine/usb"
	"machine/usb/hid/mouse"
	"runtime/interrupt"
	"runtime/volatile"
	"strconv"
	"time"

	"github.com/sago35/koebiten"
	"github.com/sago35/koebiten/games/all/all"
	"github.com/sago35/koebiten/hardware"
	keyboard "github.com/sago35/tinygo-keyboard"
	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/drivers/tone"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
)

const (
	SCREENSAVER = iota
	LAYER
)

var (
	invertRotaryPins = false
	currentLayer     = 0
	displayShowing   = SCREENSAVER
	displayFrame     = 0
	koebitenEnable   = false

	textWhite = color.RGBA{255, 255, 255, 255}
	textBlack = color.RGBA{0, 0, 0, 255}
)

func main() {
	usb.Product = "conf2025badge"

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
	//buzzer()
	i2c := machine.I2C1
	i2c.Configure(machine.I2CConfig{
		Frequency: 2.0 * machine.MHz,
		SDA:       machine.GPIO6,
		SCL:       machine.GPIO7,
	})

	display := ssd1306.NewI2C(i2c)
	display.Configure(ssd1306.Config{
		Address:  0x3C,
		Width:    128,
		Height:   64,
		Rotation: drivers.Rotation0,
	})
	display.ClearDisplay()
	displayBuffer := NewDisplayBuffer(display.Size())

	var changed volatile.Register8
	changed.Set(0)

	wsPin := machine.GPIO0
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, wsPin)
	err := ws.EnableDMA(true)
	if err != nil {
		return err
	}
	wsLeds := [2]uint32{}
	for i := range wsLeds {
		wsLeds[i] = black
	}
	writeColors(s, ws, wsLeds[:])

	{
		machine.NEO_PWR.Configure(machine.PinConfig{Mode: machine.PinOutput})
		machine.NEO_PWR.High()
		time.Sleep(500 * time.Millisecond)

		wsPin := machine.GPIO12
		s, _ := pio.PIO1.ClaimStateMachine()
		ws, _ := piolib.NewWS2812B(s, wsPin)
		err := ws.EnableDMA(true)
		if err != nil {
			return err
		}
		wsLeds := [1]uint32{}
		for i := range wsLeds {
			wsLeds[i] = black
		}
		writeColors(s, ws, wsLeds[:])
	}

	machine.InitADC()
	ax := machine.GPIO27
	ay := machine.GPIO26

	m := mouse.Port()

	d := keyboard.New()

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
	rk.SetCallback(func(layer, index int, state keyboard.State) {
	})

	gpioPins := []machine.Pin{machine.GPIO28, machine.GPIO29, machine.GPIO2}
	for c := range gpioPins {
		gpioPins[c].Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	gk := d.AddGpioKeyboard(gpioPins, [][]keyboard.Keycode{
		{
			jp.MouseRight, jp.MouseLeft, jp.KeyTo1,
		},
		{
			jp.MouseRight, jp.MouseLeft, jp.KeyTo2,
		},
		{
			jp.KeyTo5, jp.KeyTo5, jp.KeyTo0,
		},
	})
	gk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == keyboard.PressToRelease {
			if currentLayer == 5 {
				koebitenEnable = true
			}
			return
		}
		if layer != d.Layer() {
			displayFrame = 0
			currentLayer = d.Layer()
			displayShowing = LAYER
		}
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
			pixel := displayBuffer.GetPixel(dispx, dispy)
			c := textWhite
			if pixel {
				c = textBlack
			}
			displayBuffer.SetPixel(dispx, dispy, c)
			dispx += deltaX
			dispy += deltaY

			if dispx == 0 || dispx == 127 {
				deltaX = -deltaX
			}

			if dispy == 0 || dispy == 63 {
				deltaY = -deltaY
			}

			switch displayShowing {
			case LAYER:
				if currentLayer == 5 {
					if koebitenEnable {
						display.ClearDisplay()

						machine.GPIO3.SetInterrupt(machine.PinToggle, nil)
						machine.GPIO4.SetInterrupt(machine.PinToggle, nil)

						koebiten.SetHardware(hardware.Device)
						koebiten.SetRotation(koebiten.Rotation0)

						game := all.NewGame()
						if err := koebiten.RunGame(game); err != nil {
							log.Fatal(err)
						}
						game.RunCurrentGame()
					}
				} else {
					if displayFrame == 0 {
						display.ClearDisplay()
						_, w := tinyfont.LineWidth(&freemono.Regular12pt7b, "LAYER "+strconv.Itoa(currentLayer))
						tinyfont.WriteLine(display, &freemono.Regular12pt7b, int16(128-w)/2, 40, "LAYER "+strconv.Itoa(currentLayer), textWhite)
						display.Display()
					} else if displayFrame > 40 {
						display.ClearDisplay()
						display.Display()
						displayShowing = SCREENSAVER
					}
				}
			case SCREENSAVER:
				display.SetBuffer(displayBuffer.GetBuffer())
				display.Display()
			}
			displayFrame++
		}

		cnt++
	}

	return nil
}

type DisplayBuffer struct {
	buffer []byte
	width  int16
	height int16
}

func NewDisplayBuffer(width, height int16) *DisplayBuffer {
	return &DisplayBuffer{
		buffer: make([]byte, width*height/8),
		width:  width,
		height: height,
	}
}

func (d DisplayBuffer) Size() (x, y int16) {
	return d.width, d.height
}

func (d *DisplayBuffer) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return
	}
	byteIndex := x + (y/8)*d.width
	if c.R != 0 || c.G != 0 || c.B != 0 {
		d.buffer[byteIndex] |= 1 << uint8(y%8)
	} else {
		d.buffer[byteIndex] &^= 1 << uint8(y%8)
	}
}

func (d DisplayBuffer) Display() error {
	return nil
}

func (d *DisplayBuffer) GetPixel(x int16, y int16) bool {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return false
	}
	byteIndex := x + (y/8)*d.width
	return (d.buffer[byteIndex] >> uint8(y%8) & 0x1) == 1
}

func (d *DisplayBuffer) SetBuffer(buffer []byte) error {
	if len(buffer) != len(d.buffer) {
		return errBufferSize
	}
	for i := 0; i < len(d.buffer); i++ {
		d.buffer[i] = buffer[i]
	}
	return nil
}

func (d *DisplayBuffer) GetBuffer() []byte {
	return d.buffer
}

var (
	errBufferSize = errors.New("invalid size buffer")
)

func buzzer() {
	bzrPin := machine.GPIO1
	pwm := machine.PWM0
	speaker, err := tone.New(pwm, bzrPin)
	if err != nil {
		println("failed to configure PWM")
		return
	}

	song := []tone.Note{
		tone.C5,
		tone.D5,
		tone.E5,
		tone.F5,
		tone.G5,
		tone.A5,
		tone.B5,
		tone.C6,
		tone.C6,
		tone.B5,
		tone.A5,
		tone.G5,
		tone.F5,
		tone.E5,
		tone.D5,
		tone.C5,
	}

	for {
		for _, val := range song {
			speaker.SetNote(val)
			time.Sleep(time.Second / 2)
		}
	}
}
