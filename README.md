# keyboards

This repository was created to manage the circuitry and firmware for the keyboards I designed.  
The firmware is created using [sago35/tinygo-keyboard](https://github.com/sago35/tinygo-keyboard).  

## sg24

![](./images/sg24.jpg)

* [kicanvas](https://kicanvas.org/?github=https%3A%2F%2Fgithub.com%2Fsago35%2Fkeyboards%2Ftree%2Fmain%2Fsg24%2Fsg24)

## zero-kb02/

![](./images/zero-kb02.jpg)

* [kicanvas](https://kicanvas.org/?github=https%3A%2F%2Fgithub.com%2Fsago35%2Fkeyboards%2Ftree%2Fmain%2Fzero-kb02%2Fzero-kb02)
* [gerber data for jlcpcb](./zero-kb02/zero-kb02/GERBER-zero-kb02.zip)
    * For the `Mark on PCB` item, please select `Order Number(Specify Position)`
* [case (stl / 3mf)](./zero-kb02/stl/)
* workshop - https://github.com/sago35/tinygo_keeb_workshop_2024

### pinout

![](./images/pinout01.jpg)

![](./images/pinout02.png)

| Name      | Pin            | Info
|-----------|----------------|------
| VR\_BTN   | machine.GPIO0  | InputPullup
| WS2812    | machine.GPIO1  | Output
| ROT\_BTN1 | machine.GPIO2  | InputPullup
| ROT\_A1   | machine.GPIO3  | InputPullup
| ROT\_B1   | machine.GPIO4  | InputPullup
| COL1      | machine.GPIO5  | Output
| COL2      | machine.GPIO6  | Output
| COL3      | machine.GPIO7  | Output
| COL4      | machine.GPIO8  | Output
| ROW1      | machine.GPIO9  | InputPulldown
| ROW2      | machine.GPIO10 | InputPulldown
| ROW3      | machine.GPIO11 | InputPulldown
| SDA0\_TX0 | machine.GPIO12 | I2C SDA
| SCL0\_RX0 | machine.GPIO13 | I2C SCL
| EX01      | machine.GPIO14 | GPIO
| EX02      | machine.GPIO15 | GPIO
| EX03      | machine.GPIO26 | GPIO / ADC
| EX04      | machine.GPIO27 | GPIO / ADC
| VR\_Y     | machine.GPIO28 | ADC
| VR\_X     | machine.GPIO29 | ADC

### matrix and leds

The image below represents the relationship between the COL and ROW of the MATRIX wiring, as well as the order of the LEDs.
It was created by [rdon_key](https://x.com/rdon_key).
![](./images/zero-kb02-matrix-and-led.png)

## sg48key2

![](./images/sg48key2.jpg)

* [kicanvas](https://kicanvas.org/?github=https%3A%2F%2Fgithub.com%2Fsago35%2Fkeyboards%2Ftree%2Fmain%2Fsg48key2%2Fsg48key2)
  * Configuration of U3 is incorrect and needs to be fixed.

