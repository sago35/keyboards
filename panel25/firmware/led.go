package main

import (
	"image/color"
)

type SK6812 struct {
	Image [100]uint32
}

func NewSK6812() *SK6812 {
	return &SK6812{
		Image: [100]uint32{},
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
	if int(idx) < len(s.Image[:]) {
		s.Image[idx] = (uint32(c.G) << 24) + (uint32(c.R) << 16) + (uint32(c.B) << 8) + 0xFF
	}
}

func (s *SK6812) Display() error {
	return nil
}

func (s *SK6812) RawColors() []uint32 {
	return s.Image[:]
}
