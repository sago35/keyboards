package main

// モードごとの LED リング演出。メインループの奇数ティック (33ms 周期) で
// いずれかを呼んでから writeColors する。

// initBadgeLEDs はバッジ画面用の 2 色コメットの初期パターンを作る。
// バッジ画面では rotate(true) でこれを回し続ける
func initBadgeLEDs() {
	ledBuffer[0x00] = 0x02021FFF
	ledBuffer[0x01] = 0x02020FFF
	ledBuffer[0x02] = 0x020208FF
	ledBuffer[0x03] = 0x020204FF
	ledBuffer[0x04] = 0x020202FF
	ledBuffer[0x05] = 0x020202FF
	ledBuffer[0x06] = 0x020202FF
	ledBuffer[0x07] = 0x020202FF
	ledBuffer[0x08] = 0x1F0202FF
	ledBuffer[0x09] = 0x0F0202FF
	ledBuffer[0x0A] = 0x080202FF
	ledBuffer[0x0B] = 0x040202FF
	ledBuffer[0x0C] = 0x020202FF
	ledBuffer[0x0D] = 0x020202FF
	ledBuffer[0x0E] = 0x020202FF
	ledBuffer[0x0F] = 0x020202FF
}

var ledPhase = 0

// ledBreathe はタイムテーブル画面用。Go ブルーで全体がゆっくり明滅する
// (yOffsets を流用した疑似サイン波、周期 ≈ 2.1 秒)
func ledBreathe() {
	ledPhase++
	b := 9 + yOffsets[(ledPhase/2)%len(yOffsets)] // 1..17
	g := uint8(b * 3 / 4)
	bl := uint8(b)
	for i := range ledBuffer {
		ledBuffer[i] = toGGRRBBAA(g, 0, bl, 0xFF)
	}
}

// ledTwinkle はデモ画面用。ランダムな LED がきらめいて減衰していく
func ledTwinkle() {
	for i := range ledBuffer {
		v := ledBuffer[i]
		g := uint8(v>>24) * 3 / 4
		r := uint8(v>>16) * 3 / 4
		b := uint8(v>>8) * 3 / 4
		ledBuffer[i] = toGGRRBBAA(g, r, b, 0xFF)
	}
	if bkRnd()%3 == 0 {
		ledBuffer[bkRnd()%NumLEDs] = toGGRRBBAA(0x14, 0x14, 0x1C, 0xFF)
	}
}

// ledCyclone はサイクロン画面用。ゲームが計算したリング色をそのまま
// (輝度 1/8 に落として) 実 LED へ出す
func ledCyclone() {
	for i := range ledBuffer {
		c := cyColors[i]
		ledBuffer[i] = toGGRRBBAA(c[1]/8, c[0]/8, c[2]/8, 0xFF)
	}
}

// ledRainbow はブロック崩し画面用のレインボーチェイス。
// 球の数が増えるほど回転が速くなる
func ledRainbow() {
	ledPhase = (ledPhase + 2 + bkNumBall/15) % 360
	for i := 0; i < NumLEDs; i++ {
		h := (ledPhase + i*360/NumLEDs) % 360
		r, g, b := hsvToRGB(h, 255, 20)
		ledBuffer[i] = toGGRRBBAA(g, r, b, 0xFF)
	}
}
