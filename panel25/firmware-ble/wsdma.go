package main

// WS2812 の 1 ビットを 8MHz SPI の 10 ビット (1.25µs) で表し、SPIM の
// EasyDMA で一括送信する。SoftDevice の割り込みでも波形が乱れない。

import (
	"image/color"
	"machine"
)

// 80µs のリセット期間 (8MHz では 1 バイト = 1µs)。アイドル中の MOSI High を
// WS2812 が余分な 1 ビットと解釈するため、リセットはフレーム先頭に置く。
const wsResetBytes = 80

// WS2812 の 4 ビット (MSB first) を SPI の 5 バイトへ変換する表
var nibbleLUT [16][5]byte

func init() {
	// 0 は 1100000000 (H 250ns)、1 は 1111100000 (H 625ns)。
	// WS2812B データシート "Data transfer time" の規定に合わせた値。
	for v := range nibbleLUT {
		var bits uint64
		for i := 3; i >= 0; i-- {
			bits <<= 10
			if v&(1<<i) != 0 {
				bits |= 0b1111100000
			} else {
				bits |= 0b1100000000
			}
		}
		for j := 0; j < 5; j++ {
			nibbleLUT[v][j] = byte(bits >> (8 * (4 - j)))
		}
	}
}

type WSDMA struct {
	spi        *machine.SPI
	buf        []byte
	numLEDs    int
	brightness uint8
}

// NewWSDMA は dataPin (SPI の SDO) に WS2812 波形を出力するドライバを作る。
// SPIM は SCK と SDI の割り当ても必要なので、未接続のピンを渡すこと。
func NewWSDMA(spi *machine.SPI, dataPin, sckPin, sdiPin machine.Pin, numLEDs int) *WSDMA {
	spi.Configure(machine.SPIConfig{
		Frequency: 8_000_000,
		SCK:       sckPin,
		SDO:       dataPin,
		SDI:       sdiPin,
	})
	return &WSDMA{
		spi:        spi,
		buf:        make([]byte, wsResetBytes+numLEDs*30),
		numLEDs:    numLEDs,
		brightness: 0xFF,
	}
}

// SetBrightness は全体の明るさ (0-255) を設定する。
func (w *WSDMA) SetBrightness(b uint8) {
	w.brightness = b
}

func (w *WSDMA) WriteColors(colors []color.RGBA) error {
	if len(colors) > w.numLEDs {
		colors = colors[:w.numLEDs]
	}
	// バッファ先頭の wsResetBytes 分は常に 0 でリセット期間になる
	pos := wsResetBytes
	for _, c := range colors {
		// GRB の順、MSB first
		for _, ch := range [3]byte{w.scale(c.G), w.scale(c.R), w.scale(c.B)} {
			copy(w.buf[pos:], nibbleLUT[ch>>4][:])
			copy(w.buf[pos+5:], nibbleLUT[ch&0x0F][:])
			pos += 10
		}
	}
	return w.spi.Tx(w.buf, nil)
}

func (w *WSDMA) scale(v uint8) uint8 {
	return uint8((uint16(v) * uint16(w.brightness)) >> 8)
}
