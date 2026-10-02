package main

import (
	"image/color"
)

// LED ロット差の色補正係数 (実測の色合わせ候補 17、49% = 125/256)。
// 上半分 (0-49) は全体輝度を、緑が強い下半分 (50-99) は G だけを落とす。
const lotCorrection = 125

func lotScale(v uint8) uint8 {
	return uint8((uint16(v) * lotCorrection) >> 8)
}

type SK6812 struct {
	Image [100]color.RGBA
}

func NewSK6812() *SK6812 {
	return &SK6812{
		Image: [100]color.RGBA{},
	}
}

func (s *SK6812) Size() (x, y int16) {
	return 10, 10
}

func (s *SK6812) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || 10 <= x || y < 0 || 10 <= y {
		return
	}

	idx := x*5 + y
	if y >= 5 {
		idx = x*5 + (y - 5) + 50
	}
	// 58 66 70 は下半分だが上半分と同じロットの LED (実機で確認)
	if idx < 50 || idx == 58 || idx == 66 || idx == 70 {
		c = color.RGBA{R: lotScale(c.R), G: lotScale(c.G), B: lotScale(c.B), A: c.A}
	} else {
		c.G = lotScale(c.G)
	}
	if int(idx) < len(s.Image[:]) {
		s.Image[idx] = c
	}
}

func (s *SK6812) Display() error {
	return nil
}

func (s *SK6812) Colors() []color.RGBA {
	return s.Image[:]
}
