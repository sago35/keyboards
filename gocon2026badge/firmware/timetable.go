package main

import (
	"image/color"
	"strconv"
	"strings"

	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/shnm"
)

// タイムテーブル画面。U/D でカーソル移動、R/L でトラック切り替え、
// A で詳細表示、B で戻る。描画は入力があったときだけ行う。
const (
	ttSessionsPerPage = 3
	ttMarginX         = 6
	ttHeaderH         = 20
	ttBlockH          = 70
	ttLineW           = 240 - ttMarginX*2
)

var (
	ttFont   = &shnm.Shnmk12
	ttTracks = []string{"Room A", "Room B", "Workshop A", "Workshop B"}
	ttTrack  = 0
	ttCursor = 0 // ttList() 内でのカーソル位置
	ttMark   = 0 // 記憶している時刻 (0:00 からの分)。U/D での移動時だけ更新し、
	// トラック切替時はこの時刻以降で最初のイベントに飛ぶ
	ttDetail = false // true なら詳細画面
	ttScroll = 0     // 詳細画面のスクロール位置 (行単位)
	ttDirty  = true  // 再描画が必要か

	ttColHeader  = color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF} // 水色の帯の上なので黒
	ttColTime    = color.RGBA{R: 0x00, G: 0xAD, B: 0xD8, A: 0xFF} // Go ブランドカラー
	ttColTitle   = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	ttColSpeaker = color.RGBA{R: 0x90, G: 0x90, B: 0x90, A: 0xFF}
	ttColDesc    = color.RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
)

// ttBackTrack はトラックリング上の「戻る」タイルの仮想 index。
// リングは [Room A, Room B, Workshop A, Workshop B, 戻る] のループで、
// Room A の左 (および Workshop B の右) が戻るタイルになる
const ttBackTrack = 4

// ttOnBack は「戻る」タイルを表示中かどうか
func ttOnBack() bool {
	return ttTrack == ttBackTrack
}

// enterTimetable はモード開始時に呼ぶ (トラックとカーソルは前回位置を保持する)
func enterTimetable() {
	ttDetail = false
	if ttOnBack() {
		ttTrack = 0
	}
	ttDirty = true
}

// ttList は現在のトラックに表示するセッションの index 一覧。
// 各トラックは独立で、track が空のもの (オープニング等の共通枠) は Room A にだけ出す
func ttList() []int {
	name := ttTracks[ttTrack]
	list := make([]int, 0, len(sessions))
	for i, s := range sessions {
		if s.track == name || (name == "Room A" && s.track == "") {
			list = append(list, i)
		}
	}
	return list
}

// ttStart はセッションの開始時刻を 0:00 からの分で返す ("HH:MM-HH:MM" 形式が前提)
func ttStart(idx int) int {
	t := sessions[idx].time
	if len(t) < 5 {
		return 0
	}
	return int(t[0]-'0')*600 + int(t[1]-'0')*60 + int(t[3]-'0')*10 + int(t[4]-'0')
}

// ttMove は U/D によるカーソル移動 (端で止まる)。移動した時刻を記憶する
func ttMove(d int) {
	if ttOnBack() {
		return
	}
	list := ttList()
	c := ttCursor + d
	if c < 0 {
		c = 0
	}
	if c > len(list)-1 {
		c = len(list) - 1
	}
	if c != ttCursor {
		ttCursor = c
		ttMark = ttStart(list[c])
		ttDirty = true
	}
}

// ttSwitchTrack は R/L によるトラック切り替え (ループ)。詳細画面では無視する。
// 切替先では記憶時刻 (ttMark) 以降で最初のイベントに飛ぶ (無ければ末尾)。
// ttMark は更新しないので、移動せずに戻れば元のセッションに戻る
func ttSwitchTrack(d int) {
	if ttDetail {
		return
	}
	n := len(ttTracks) + 1 // 実トラック + 戻るタイル
	ttTrack = (ttTrack + d + n) % n
	if !ttOnBack() {
		list := ttList()
		ttCursor = len(list) - 1
		for i, idx := range list {
			if ttStart(idx) >= ttMark {
				ttCursor = i
				break
			}
		}
	}
	ttDirty = true
}

// ttSelect は A による詳細表示のトグル
func ttSelect() {
	ttDetail = !ttDetail
	ttScroll = 0
	ttDirty = true
}

// ttUp / ttDown は U/D の動作。リストではカーソル移動、詳細では本文スクロール
func ttUp() {
	if ttDetail {
		ttScrollBy(-ttScrollStep)
	} else {
		ttMove(-1)
	}
}

func ttDown() {
	if ttDetail {
		ttScrollBy(ttScrollStep)
	} else {
		ttMove(+1)
	}
}

func ttScrollBy(d int) {
	max := len(ttBodyLines()) - ttBodyVisible()
	if max < 0 {
		max = 0
	}
	s := ttScroll + d
	if s < 0 {
		s = 0
	}
	if s > max {
		s = max
	}
	if s != ttScroll {
		ttScroll = s
		ttDirty = true
	}
}

// updateTimetable は 30Hz で呼ばれ、入力で dirty になったときだけ再描画する
func updateTimetable(display st7789.Device) error {
	if !ttDirty {
		return nil
	}
	ttDirty = false
	if ttOnBack() {
		return renderTimetableBack(display)
	}
	if ttDetail {
		return renderTimetableDetail(display)
	}
	return renderTimetableList(display)
}

// renderTimetableBack は「戻る」タイルの画面
func renderTimetableBack(display st7789.Device) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	d := ttClear("戻る", "")
	tinyfont.WriteLine(d, ttFont, ttMarginX, 116, "A: バッジ画面に戻る", ttColTitle)
	tinyfont.WriteLine(d, ttFont, ttMarginX, 140, "L/R: タイムテーブルへ", ttColSpeaker)
	return display.DrawBitmap(0, 0, pixelBuf)
}

// ttRuneWidth は shnm12 での表示幅 (半角 6px, 全角 12px)
func ttRuneWidth(r rune) int {
	if r < 0x80 {
		return 6
	}
	return 12
}

// ttSplit は幅 maxw に収まる先頭部分と残りを返す
func ttSplit(s string, maxw int) (head, rest string) {
	w := 0
	for i, r := range s {
		rw := ttRuneWidth(r)
		if w+rw > maxw {
			return s[:i], s[i:]
		}
		w += rw
	}
	return s, ""
}

// ttEllipsis は幅 maxw に収まらない場合に … で切り詰める
func ttEllipsis(s string, maxw int) string {
	if _, rest := ttSplit(s, maxw); rest == "" {
		return s
	}
	head, _ := ttSplit(s, maxw-12)
	return head + "…"
}

// ttClear は pixelBuf を黒で塗りつぶし、ヘッダ帯とテキストを描く
func ttClear(left, right string) *imageDisplayer {
	raw := pixelBuf.RawBuffer()
	for i := range raw {
		raw[i] = 0
	}

	headerBG := pixel.NewColor[pixel.RGB565BE](0x00, 0xAD, 0xD8)
	for y := 0; y < ttHeaderH; y++ {
		for x := 0; x < 240; x++ {
			pixelBuf.Set(x, y, headerBG)
		}
	}
	d := &imageDisplayer{img: pixelBuf}
	tinyfont.WriteLine(d, ttFont, ttMarginX, 14, left, ttColHeader)
	tinyfont.WriteLine(d, ttFont, int16(240-ttMarginX-len(right)*6), 14, right, ttColHeader)
	return d
}

func renderTimetableList(display st7789.Device) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	list := ttList()
	pos := strconv.Itoa(ttCursor+1) + "/" + strconv.Itoa(len(list))
	d := ttClear(ttTracks[ttTrack], pos)

	page := ttCursor / ttSessionsPerPage
	cursorBG := pixel.NewColor[pixel.RGB565BE](0x00, 0x28, 0x38)
	cursorBar := pixel.NewColor[pixel.RGB565BE](0x00, 0xAD, 0xD8)

	for i := 0; i < ttSessionsPerPage; i++ {
		idx := page*ttSessionsPerPage + i
		if idx >= len(list) {
			break
		}
		s := sessions[list[idx]]
		y := int16(26 + i*ttBlockH)

		if idx == ttCursor {
			// カーソル: 下地の帯 + 左端のバー
			for yy := int(y) - 4; yy < int(y)+60; yy++ {
				for xx := 0; xx < 240; xx++ {
					pixelBuf.Set(xx, yy, cursorBG)
				}
				pixelBuf.Set(0, yy, cursorBar)
				pixelBuf.Set(1, yy, cursorBar)
				pixelBuf.Set(2, yy, cursorBar)
			}
		}

		tinyfont.WriteLine(d, ttFont, ttMarginX, y+11, ttEllipsis(s.time+" "+s.track, ttLineW), ttColTime)

		l1, rest := ttSplit(s.title, ttLineW)
		tinyfont.WriteLine(d, ttFont, ttMarginX, y+25, l1, ttColTitle)
		if rest != "" {
			tinyfont.WriteLine(d, ttFont, ttMarginX, y+38, ttEllipsis(rest, ttLineW), ttColTitle)
		}

		tinyfont.WriteLine(d, ttFont, ttMarginX, y+52, ttEllipsis(s.speaker, ttLineW), ttColSpeaker)
	}

	return display.DrawBitmap(0, 0, pixelBuf)
}

// 詳細画面のレイアウト
const (
	ttDetailTop   = 26 // ヘッダ帯の下 (タイトル固定部の開始 y)
	ttDetailLineH = 14
	ttScrollStep  = 1 // U/D 1 回のスクロール行数
)

type ttLine struct {
	text string
	col  color.RGBA
}

func ttCurrentSession() session {
	list := ttList()
	if ttCursor >= len(list) {
		ttCursor = len(list) - 1
	}
	return sessions[list[ttCursor]]
}

// ttFixedLines は詳細画面でスクロールしない部分 (タイトル + 登壇者)
func ttFixedLines() []ttLine {
	s := ttCurrentSession()
	lines := make([]ttLine, 0, 4)
	rest := s.title
	for rest != "" {
		var line string
		line, rest = ttSplit(rest, ttLineW)
		lines = append(lines, ttLine{line, ttColTitle})
	}
	if s.speaker != "" {
		lines = append(lines, ttLine{s.speaker, ttColSpeaker})
	}
	return lines
}

// ttBodyLines はスクロール対象の本文。\n は元ページの改行 (空行 = 段落の区切り)
func ttBodyLines() []ttLine {
	s := ttCurrentSession()
	lines := make([]ttLine, 0, 16)
	for _, para := range strings.Split(s.desc, "\n") {
		if para == "" {
			lines = append(lines, ttLine{"", ttColDesc})
			continue
		}
		rest := para
		for rest != "" {
			var line string
			line, rest = ttSplit(rest, ttLineW)
			lines = append(lines, ttLine{line, ttColDesc})
		}
	}
	return lines
}

// ttBodyTop は本文領域の開始 y (固定部の行数 + 区切り線ぶん)
func ttBodyTop() int {
	return ttDetailTop + len(ttFixedLines())*ttDetailLineH + 6
}

// ttBodyVisible は本文領域に入る行数
func ttBodyVisible() int {
	return (240 - ttBodyTop()) / ttDetailLineH
}

func renderTimetableDetail(display st7789.Device) error {
	// 前の DMA 転送が pixelBuf を読んでいる間は書き換えない
	spiBus.Wait()

	s := ttCurrentSession()
	d := ttClear(s.time, s.track)

	// 固定部: タイトル + 登壇者
	fixed := ttFixedLines()
	for i, l := range fixed {
		y := int16(ttDetailTop + i*ttDetailLineH + 11)
		tinyfont.WriteLine(d, ttFont, ttMarginX, y, l.text, l.col)
	}

	// 固定部と本文の区切り線
	bodyTop := ttBodyTop()
	sep := pixel.NewColor[pixel.RGB565BE](0x40, 0x40, 0x40)
	for x := 0; x < 240; x++ {
		pixelBuf.Set(x, bodyTop-3, sep)
	}

	// 本文 (スクロール対象)
	body := ttBodyLines()
	visible := ttBodyVisible()
	if max := len(body) - visible; ttScroll > max {
		if max < 0 {
			max = 0
		}
		ttScroll = max
	}
	for i := 0; i < visible && ttScroll+i < len(body); i++ {
		l := body[ttScroll+i]
		if l.text == "" {
			continue
		}
		y := int16(bodyTop + i*ttDetailLineH + 11)
		tinyfont.WriteLine(d, ttFont, ttMarginX, y, l.text, l.col)
	}

	// スクロールバー (本文領域のみ。全行が収まるときは出さない)
	if len(body) > visible {
		barBG := pixel.NewColor[pixel.RGB565BE](0x20, 0x20, 0x20)
		barFG := pixel.NewColor[pixel.RGB565BE](0x00, 0xAD, 0xD8)
		area := 240 - bodyTop
		thumbH := area * visible / len(body)
		thumbY := bodyTop + area*ttScroll/len(body)
		for yy := bodyTop; yy < 240; yy++ {
			c := barBG
			if yy >= thumbY && yy < thumbY+thumbH {
				c = barFG
			}
			pixelBuf.Set(238, yy, c)
			pixelBuf.Set(239, yy, c)
		}
	}

	return display.DrawBitmap(0, 0, pixelBuf)
}
