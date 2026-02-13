//go:build invert_diode

package main

func init() {
	// This is the configuration for when the diode is attached in reverse compared to schema.
	// You can enable it by specifying `--tags invert_diode`.
	indexMapFunc = func(i, j, row, col, numPins int) int {
		if i > j {
			i--
		}
		return i*numPins + j
	}
}
