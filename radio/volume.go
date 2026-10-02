package main

func dacLevel(v int) int {
	v = clamp(v, 0, 100)
	if v == 0 {
		return 0
	}
	// DAC steps are 0.5 dB; a linear 0..190 mapping makes midrange settings nearly silent.
	return 130 + v*50/100
}
