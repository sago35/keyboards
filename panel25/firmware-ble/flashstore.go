package main

// デフォルトのスクロール文字列を内蔵フラッシュへ保存する。
// machine.Flash は SoftDevice 有効時に sd_flash_* を使うので BLE 中でも安全。
// 領域はファームウェア末尾の空きページなので、書き込み直すと消える。

import "machine"

const (
	flashMagic  = "P25T"
	flashMaxLen = 250
)

// loadDefaultText は保存済みの文字列を返す。未保存なら空文字を返す
func loadDefaultText() string {
	var hdr [6]byte
	_, err := machine.Flash.ReadAt(hdr[:], 0)
	if err != nil || string(hdr[0:4]) != flashMagic {
		return ""
	}
	n := int(hdr[4]) | int(hdr[5])<<8
	if n == 0 || n > flashMaxLen {
		return ""
	}
	buf := make([]byte, n)
	_, err = machine.Flash.ReadAt(buf, 6)
	if err != nil {
		return ""
	}
	return string(buf)
}

// saveDefaultText は文字列を保存する。空文字で保存すると未保存扱いに戻る
func saveDefaultText(s string) error {
	if len(s) > flashMaxLen {
		s = s[:flashMaxLen]
	}
	err := machine.Flash.EraseBlocks(0, 1)
	if err != nil {
		return err
	}
	buf := make([]byte, 6+len(s))
	copy(buf[0:4], flashMagic)
	buf[4] = byte(len(s))
	buf[5] = byte(len(s) >> 8)
	copy(buf[6:], s)
	_, err = machine.Flash.WriteAt(buf, 0)
	return err
}
