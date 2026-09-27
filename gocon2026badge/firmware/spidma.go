//go:build rp2040

package main

import (
	"device/rp"
	"machine"
	"runtime"
	"runtime/volatile"
	"unsafe"
)

// dmaSPI は machine.SPI をラップし、大きな書き込みを DMA で非同期送信する。
// Tx から戻った時点では転送が続いていることがあるため、CS はドライバに渡さず
// 常時 Low に固定し、DC を切り替える操作の前に必ず Wait を呼ぶこと。
type dmaSPI struct {
	bus     *machine.SPI
	cs      machine.Pin
	ch      *dmaChannelHW
	pending []byte // DMA 転送中のバッファ (完了待ち判定と GC 保護を兼ねる)
}

const (
	// machine が SPI0/SPI1 用に ch0/ch1 を、piolib が若い番号から claim するため、
	// 衝突しない高い番号を固定で使う
	spiDMAChannel = 11

	// これ未満の書き込みは DMA セットアップの方が高くつくので CPU で送る
	spiDMAMinLen = 512
)

// DMA チャンネルのレジスタ配置 (RP2040 Datasheet 2.5.7)
type dmaChannelHW struct {
	READ_ADDR   volatile.Register32
	WRITE_ADDR  volatile.Register32
	TRANS_COUNT volatile.Register32
	CTRL_TRIG   volatile.Register32
	_           [12]volatile.Register32 // エイリアスレジスタ
}

var dmaChannels = (*[12]dmaChannelHW)(unsafe.Pointer(rp.DMA))

// newDMASPI は bus をラップする dmaSPI を返す。CS はラッパーが管理する
// (ST7789 は CS の立ち下がりエッジでビット同期するため、転送単位でトグルしつつ
// 非同期転送中だけ Low を保持する必要がある)。ドライバ側には machine.NoPin を渡すこと。
func newDMASPI(bus *machine.SPI, cs machine.Pin) *dmaSPI {
	cs.Configure(machine.PinConfig{Mode: machine.PinOutput})
	cs.High()
	return &dmaSPI{bus: bus, cs: cs, ch: &dmaChannels[spiDMAChannel]}
}

// Wait は非同期転送の完了を待つ。DMA が FIFO に詰め終わった後、実際に
// 線上へシフトアウトし終わるまで待ち、読み捨てで溢れた RX 側を掃除する。
func (d *dmaSPI) Wait() {
	if d.pending == nil {
		return
	}
	for d.ch.CTRL_TRIG.Get()&rp.DMA_CH0_CTRL_TRIG_BUSY != 0 {
		runtime.Gosched()
	}
	for d.bus.Bus.SSPSR.HasBits(rp.SPI0_SSPSR_RNE) {
		d.bus.Bus.SSPDR.Get()
	}
	for d.bus.Bus.SSPSR.HasBits(rp.SPI0_SSPSR_BSY) {
		runtime.Gosched()
	}
	for d.bus.Bus.SSPSR.HasBits(rp.SPI0_SSPSR_RNE) {
		d.bus.Bus.SSPDR.Get()
	}
	d.bus.Bus.SSPICR.Set(rp.SPI0_SSPICR_RORIC)
	d.pending = nil
	d.cs.High()
}

func (d *dmaSPI) Tx(w, r []byte) error {
	d.Wait()
	if r != nil || len(w) < spiDMAMinLen {
		d.cs.Low()
		err := d.bus.Tx(w, r)
		d.cs.High()
		return err
	}

	dreq := uint32(rp.DREQ_SPI0_TX)
	if d.bus.Bus == rp.SPI1 {
		dreq = rp.DREQ_SPI1_TX
	}

	// 非同期転送中は CS を Low に保ち、Wait 完了時に High へ戻す
	d.pending = w
	d.cs.Low()
	d.ch.READ_ADDR.Set(uint32(uintptr(unsafe.Pointer(&w[0]))))
	d.ch.WRITE_ADDR.Set(uint32(uintptr(unsafe.Pointer(&d.bus.Bus.SSPDR))))
	d.ch.TRANS_COUNT.Set(uint32(len(w)))
	d.ch.CTRL_TRIG.Set(rp.DMA_CH0_CTRL_TRIG_INCR_READ |
		rp.DMA_CH0_CTRL_TRIG_DATA_SIZE_SIZE_BYTE<<rp.DMA_CH0_CTRL_TRIG_DATA_SIZE_Pos |
		dreq<<rp.DMA_CH0_CTRL_TRIG_TREQ_SEL_Pos |
		spiDMAChannel<<rp.DMA_CH0_CTRL_TRIG_CHAIN_TO_Pos | // 自分自身を指定してチェイン無効化
		rp.DMA_CH0_CTRL_TRIG_EN)
	return nil
}

func (d *dmaSPI) Transfer(b byte) (byte, error) {
	d.Wait()
	d.cs.Low()
	v, err := d.bus.Transfer(b)
	d.cs.High()
	return v, err
}
