package main

import (
	_ "embed"
	"fmt"
	"image/color"
	"log"
	"machine"
	"machine/usb"
	"time"

	keyboard "github.com/sago35/tinygo-keyboard"
	"tinygo.org/x/drivers/ws2812"
)

func main() {
	usb.Product = "sg24-0.1.0"

	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	machine.InitADC()
	ax := machine.A0
	ay := machine.A1

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
			0x0000, 0x0001, 0x0002, 0x0003, 0x0004, 0x0005,
			0x0006, 0x0007, 0x0008, 0x0009, 0x000A, 0x000B,
			0x000C, 0x000D, 0x000E, 0x000F, 0x0010, 0x0011,
			0x0012, 0x0013, 0x0014, 0x0015, 0x0016, 0x0017,
		},
	})
	sm.SetCallback(func(layer, index int, state keyboard.State) {
		layer = d.Layer()
		fmt.Printf("sm: %d %d %d\n", layer, index, state)
	})

	neo := machine.NEOPIXEL
	neo.Configure(machine.PinConfig{Mode: machine.PinOutput})
	ws := ws2812.New(neo)
	ws.WriteColors([]color.RGBA{{R: 0x33, G: 0x33, B: 0x33}})

	uart := machine.UART0
	uart.Configure(machine.UARTConfig{TX: machine.UART0_TX_PIN, RX: machine.NoPin})

	d.Keyboard = &keyboard.UartTxKeyboard{
		Uart: uart,
	}

	err := d.Init()
	if err != nil {
		return err
	}

	cont := true
	x := NewADCDevice(ax, 0x2800, 0xD600, true)
	y := NewADCDevice(ay, 0x2000, 0xCC00, true)
	ticker := time.Tick(1 * time.Millisecond)
	cnt := 0
	for cont {
		<-ticker
		err := d.Tick()
		if err != nil {
			return err
		}

		if cnt%10 == 0 {
			xx := x.Get2()
			yy := y.Get2()
			if xx != 0 || yy != 0 {
				fmt.Printf("%04X %04X %4d %4d %4d %4d\n", x.RawValue, y.RawValue, xx, yy, x.Get(), y.Get())
				uart.Write([]byte{0xF0, byte(xx), byte(yy)})
				//m.Move(int(xx), int(yy))
			}
		}
		cnt++
	}

	return nil
}
