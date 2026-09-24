package main

// panel25 を xiao-ble から BLE (Nordic UART Service) で操作する版。
// 基本は固定文言をスクロールし続け、BLE で受けた内容 (文字列またはピクセル
// 編集) に割り込まれる。0x04 か切断で先頭からのスクロールに戻る。
// 復帰タイミングなどの制御は webble/index.html 側が持つ。

import (
	"image/color"
	"machine"
	"runtime/interrupt"
	"time"

	"github.com/mattn/go-runewidth"
	"tinygo.org/x/bluetooth"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/shnm"
)

var (
	// panel25 の WS2812 データ線をつなぐ xiao-ble のピン
	wsPin = machine.D7
	// SPIM は SCK と SDI の割り当ても要求するため未接続のピンを充てる
	wsSckPin = machine.D8
	wsSdiPin = machine.D9
)

const (
	// true にすると BLE を起動せず、全 LED を白で点滅させる
	// 配線確認テスト (ledtest.go) だけを行う
	ledTest = false

	deviceName = "panel25-ble"
	// 100 個の LED を全灯させない安全側の明るさ (0-255)
	brightness  = 0x40
	defaultText = "TinyGo Conference 2026 in JAPAN"
	maxMsgLen   = 256

	// ピクセル編集コマンド。0x01 x y r g b の 6 バイト、0x02 は全消去、
	// 0x04 はデフォルトのスクロール表示へ戻す
	cmdSetPixel = 0x01
	cmdClear    = 0x02
	cmdReturn   = 0x04
)

// BLE の WriteEvent は割り込みコンテキストで呼ばれるため、
// 固定長バッファへのコピーだけを行い、本処理はメインループ側で行う。
var (
	rxBuf       [maxMsgLen]byte
	rxLen       int
	rxSeq       uint8
	rxCommitted bool

	// ピクセル編集用のフレームバッファ (y*10+x)
	editImage [100]color.RGBA
	editSeq   uint8
	returnReq bool
)

func onRxWrite(_ bluetooth.Connection, _ int, value []byte) {
	if len(value) > 0 && (value[0] == cmdSetPixel || value[0] == cmdClear || value[0] == cmdReturn) {
		onPixelCommand(value)
		return
	}

	if rxCommitted {
		rxLen = 0
		rxCommitted = false
	}
	for _, b := range value {
		if b == '\n' || b == '\r' {
			if rxLen > 0 {
				rxCommitted = true
			}
			continue
		}
		if rxLen < maxMsgLen {
			rxBuf[rxLen] = b
			rxLen++
		}
	}
	rxSeq++
}

// onPixelCommand も割り込みコンテキストで呼ばれるため、変数の代入だけを行う
func onPixelCommand(value []byte) {
	for len(value) > 0 {
		switch value[0] {
		case cmdSetPixel:
			if len(value) < 6 {
				return
			}
			x, y := value[1], value[2]
			if x < 10 && y < 10 {
				editImage[int(y)*10+int(x)] = color.RGBA{R: value[3], G: value[4], B: value[5]}
			}
			value = value[6:]
		case cmdClear:
			for i := range editImage {
				editImage[i] = color.RGBA{}
			}
			value = value[1:]
		case cmdReturn:
			returnReq = true
			value = value[1:]
			continue
		default:
			return
		}
		editSeq++
	}
}

func main() {
	if ledTest {
		runLedTest()
	}

	run()
}

func run() {
	// SoftDevice と共存させるため SPIM + EasyDMA で駆動する (wsdma.go)
	ws := NewWSDMA(machine.SPI0, wsPin, wsSckPin, wsSdiPin, 100)
	ws.SetBrightness(brightness)

	display := NewSK6812()

	// デフォルトのスクロール文字列。フラッシュに保存済みならそちらを使う
	defText := defaultText
	if s := loadDefaultText(); s != "" {
		defText = s
	}

	str := defText
	// BLE の初期化に失敗しても LED 表示は続け、エラー内容をスクロール表示する
	err := enableBLE()
	if err != nil {
		str = "BLE ERR: " + err.Error()
	}

	defaultColor := color.RGBA{R: 0x00, G: 0xFF, B: 0x00}
	textColor := defaultColor
	pendingColor := defaultColor
	width := int16(runewidth.StringWidth(str))
	cnt := int16(0)

	lastSeq := rxSeq
	lastChange := time.Now()

	editLastSeq := editSeq
	bleActive := false
	showEdit := false

	ticker := time.Tick(100 * time.Millisecond)
	for {
		<-ticker

		// 戻る指示 (0x04 または切断) でデフォルト表示を先頭から再開する
		if returnReq {
			returnReq = false
			if bleActive {
				bleActive = false
				showEdit = false
				str = defText
				textColor = defaultColor
				pendingColor = defaultColor
				width = int16(runewidth.StringWidth(str))
				cnt = 0
			}
		}

		// BLE で受信したメッセージを反映する。"#RRGGBB" は次の文字列の色
		if seq := rxSeq; seq != lastSeq {
			lastSeq = seq
			lastChange = time.Now()

			state := interrupt.Disable()
			msg := string(rxBuf[:rxLen])
			interrupt.Restore(state)

			if len(msg) > 0 && msg[0] == '=' {
				// "=文字列" でデフォルトを変更してフラッシュへ保存する。
				// "=" だけなら組み込みの文字列に戻す
				newDef := msg[1:]
				if newDef == "" {
					defText = defaultText
				} else {
					defText = newDef
				}
				saveDefaultText(newDef)
				bleActive = false
				showEdit = false
				str = defText
				textColor = defaultColor
				width = int16(runewidth.StringWidth(str))
				cnt = 0
			} else if c, ok := parseColor(msg); ok {
				pendingColor = c
			} else if msg != "" {
				str = msg
				textColor = pendingColor
				width = int16(runewidth.StringWidth(str))
				cnt = 0
				bleActive = true
				showEdit = false
			}
			txChar.Write([]byte(str + "\n"))
		} else if !rxCommitted && time.Since(lastChange) > time.Second {
			// 改行なしのまま 1 秒経ったら確定扱いにし、
			// 次の書き込みを新しいメッセージとして受け付ける
			state := interrupt.Disable()
			rxCommitted = rxLen > 0
			interrupt.Restore(state)
		}

		// ピクセル編集コマンドを受けたら編集画面の表示に切り替える
		if seq := editSeq; seq != editLastSeq {
			editLastSeq = seq
			bleActive = true
			showEdit = true
		}

		if showEdit {
			for i := int16(0); i < 100; i++ {
				display.SetPixel(i%10, i/10, editImage[i])
			}
			ws.WriteColors(display.Colors())
			continue
		}

		for i := int16(0); i < 100; i++ {
			display.SetPixel(i%10, i/10, color.RGBA{})
		}
		tinyfont.WriteLine(display, &shnm.Shnmk12, 10+cnt*-1, 9, str, textColor)
		ws.WriteColors(display.Colors())
		cnt = (cnt + 1) % (width*7 + 8)
	}
}

var txChar bluetooth.Characteristic

func enableBLE() error {
	adapter := bluetooth.DefaultAdapter
	err := adapter.Enable()
	if err != nil {
		return err
	}

	// 切断されたらデフォルト表示に戻す (アドバタイズは自動で再開される)
	adapter.SetConnectHandler(func(_ bluetooth.Device, connected bool) {
		if !connected {
			returnReq = true
		}
	})

	var rxChar bluetooth.Characteristic
	err = adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDNordicUART,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle:     &rxChar,
				UUID:       bluetooth.CharacteristicUUIDUARTRX,
				Flags:      bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteEvent: onRxWrite,
			},
			{
				Handle: &txChar,
				UUID:   bluetooth.CharacteristicUUIDUARTTX,
				Flags:  bluetooth.CharacteristicNotifyPermission | bluetooth.CharacteristicReadPermission,
			},
		},
	})
	if err != nil {
		return err
	}

	adv := adapter.DefaultAdvertisement()
	// レガシーアドバタイズは 31 バイト上限 (Bluetooth Core Spec Vol 6 Part B
	// 2.3) のため 128bit UUID は載せない。サービス発見には影響しない。
	err = adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: deviceName,
	})
	if err != nil {
		return err
	}
	return adv.Start()
}

// parseColor は "#RRGGBB" 形式の文字列を色として解釈する
func parseColor(s string) (color.RGBA, bool) {
	if len(s) != 7 || s[0] != '#' {
		return color.RGBA{}, false
	}
	var v [6]uint8
	for i := 0; i < 6; i++ {
		c := s[i+1]
		switch {
		case '0' <= c && c <= '9':
			v[i] = c - '0'
		case 'a' <= c && c <= 'f':
			v[i] = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			v[i] = c - 'A' + 10
		default:
			return color.RGBA{}, false
		}
	}
	return color.RGBA{
		R: v[0]<<4 | v[1],
		G: v[2]<<4 | v[3],
		B: v[4]<<4 | v[5],
	}, true
}
