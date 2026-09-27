package main

import (
	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/st7789"
)

// gopher は右上の画面外から中央へ飛来し、しばらく上下に漂った後、
// 左へ抜けて画面外でひと休みしてから繰り返す。
// マーキー帯 (y >= marqueeY) には入らないこと。
const (
	gopherW = 100
	gopherH = 100

	gopherHomeX = 60 // 漂うときの位置
	gopherHomeY = 60

	gopherEnterSteps = 90                    // 進入にかける更新回数 (66.7ms/回 ≈ 6 秒)
	gopherBobSteps   = 64                    // 漂う時間 (yOffsets 2 周 ≈ 4.3 秒)
	gopherExitSteps  = 45                    // 退場にかける更新回数 ≈ 3 秒
	gopherExitDist   = gopherHomeX + gopherW // 退場の移動距離 (画面外まで)
	gopherWaitSteps  = 15                    // 画面外での待機 ≈ 1 秒
)

type gopherPhase int

const (
	gopherEnter gopherPhase = iota
	gopherBob
	gopherExit
	gopherWait
)

var (
	gopherState       = gopherEnter
	gopherStep        = 0
	gopherOsc         = 0 // 揺れの位相。出現から退場まで共通に回し続ける
	gopherX           = 240
	gopherY           = -gopherH
	gopherPrevY       = -gopherH
	gopherPrevVisible = false
)

// 上下の漂い (32 分割の疑似サイン波)
var yOffsets = [...]int{
	0, 2, 3, 4, 6, 7, 7, 8,
	8, 8, 7, 7, 6, 4, 3, 2,
	0, -2, -3, -4, -6, -7, -7, -8,
	-8, -8, -7, -7, -6, -4, -3, -2,
}

// gopherSwayX / gopherSwayY は共通の揺れ (横 ±4px, 縦 ±8px、位相 1/4 ずれの楕円)。
// 出現から退場まで同じ揺れを重ね続けることで、フェーズの継ぎ目を感じさせない
func gopherSwayX() int {
	return yOffsets[(gopherOsc+8)%len(yOffsets)] / 2
}

func gopherSwayY() int {
	return yOffsets[gopherOsc%len(yOffsets)]
}

// updateGopher はアニメーションを 1 ステップ進めて描画する。
// 揺れは常に一定で、ベース軌道だけをイーズイン/アウトで滑らかに変える:
// 進入は中央に向けて減速 (到着時に基準速度 0 = そのまま漂いへ)、
// 退場は速度 0 から加速して左へ抜ける
func updateGopher(display st7789.Device) error {
	gopherOsc++
	switch gopherState {
	case gopherEnter:
		// 右上 (240, -100) から中央へイーズアウトで入る
		n := gopherEnterSteps
		p := gopherStep
		if p > n {
			p = n
		}
		f := 2*n*p - p*p // 0 → n² (二次イーズアウト)
		gopherX = 240 + (gopherHomeX-240)*f/(n*n) + gopherSwayX()
		gopherY = -gopherH + (gopherHomeY+gopherH)*f/(n*n) + gopherSwayY()
		if gopherStep >= n {
			gopherState = gopherBob
			gopherStep = 0
		}
	case gopherBob:
		gopherX = gopherHomeX + gopherSwayX()
		gopherY = gopherHomeY + gopherSwayY()
		if gopherStep >= gopherBobSteps {
			gopherState = gopherExit
			gopherStep = 0
		}
	case gopherExit:
		// 中央からイーズインで加速しながら左へ抜ける
		n := gopherExitSteps
		p := gopherStep
		gopherX = gopherHomeX - gopherExitDist*p*p/(n*n) + gopherSwayX()
		gopherY = gopherHomeY + gopherSwayY()
		if gopherX <= -gopherW {
			gopherState = gopherWait
			gopherStep = 0
		}
	case gopherWait:
		if gopherStep >= gopherWaitSteps {
			gopherState = gopherEnter
			gopherStep = 0
			gopherX = 240
			gopherY = -gopherH
		}
	}
	gopherStep++

	return drawGopher(display)
}

// overlayGopher は pixelBuf へ gopher を透過合成する (0x0000 を透明色とみなす)。
// 画面外とマーキー帯へのはみ出しはクリップする。
func overlayGopher(xofs, yofs int) {
	b := gopher565
	for y := 0; y < gopherH; y++ {
		sy := y + yofs
		if sy < 0 || sy >= marqueeY {
			continue
		}
		for x := 0; x < gopherW; x++ {
			sx := x + xofs
			if sx < 0 || sx >= 240 {
				continue
			}
			p := (uint16(b[(x+y*gopherW)*2+1]) << 8) + uint16(b[(x+y*gopherW)*2+0])
			if p != 0x0000 {
				pixelBuf.Set(sx, sy, pixel.RGB565BE(p))
			}
		}
	}
}

// drawGopher は前回位置と今回位置を覆う帯だけを背景から再合成して転送する
func drawGopher(display st7789.Device) error {
	visible := gopherX > -gopherW && gopherX < 240 &&
		gopherY > -gopherH && gopherY < marqueeY
	if !visible && !gopherPrevVisible {
		// 画面外に居続けている間は転送しない
		gopherPrevY = gopherY
		return nil
	}

	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	top := gopherY
	if gopherPrevY < top {
		top = gopherPrevY
	}
	bottom := gopherY + gopherH
	if gopherPrevY+gopherH > bottom {
		bottom = gopherPrevY + gopherH
	}
	if top < 0 {
		top = 0
	}
	if bottom > marqueeY {
		bottom = marqueeY
	}
	gopherPrevY = gopherY
	gopherPrevVisible = visible
	if bottom <= top {
		return nil
	}

	raw := pixelBuf.RawBuffer()
	copy(raw[top*240*2:bottom*240*2], background565[top*240*2:bottom*240*2])
	overlayGopher(gopherX, gopherY)

	band := pixel.NewImageFromBytes[pixel.RGB565BE](240, bottom-top, raw[top*240*2:bottom*240*2])
	return display.DrawBitmap(0, int16(top), band)
}
