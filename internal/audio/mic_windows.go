//go:build windows

package audio

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	winmm         = syscall.NewLazyDLL("winmm.dll")
	waveInOpen    = winmm.NewProc("waveInOpen")
	waveInPrepare = winmm.NewProc("waveInPrepareHeader")
	waveInAdd     = winmm.NewProc("waveInAddBuffer")
	waveInStart   = winmm.NewProc("waveInStart")
	waveInReset   = winmm.NewProc("waveInReset")
	waveInUnprep  = winmm.NewProc("waveInUnprepareHeader")
	waveInClose   = winmm.NewProc("waveInClose")
)

const (
	waveMapper = 0xFFFFFFFF
	whdrDone   = 0x1
	buffers    = 8
	bufBytes   = 4096
)

// waveFormat mirrors WAVEFORMATEX (18 bytes used).
type waveFormat struct {
	tag, channels   uint16
	rate, byteRate  uint32
	align, bits, cb uint16
}

// waveHdr mirrors WAVEHDR.
type waveHdr struct {
	data           *byte
	length, got    uint32
	user           uintptr
	flags, loops   uint32
	next, reserved uintptr
}

type waveIn struct {
	h    atomic.Uintptr // 0 once closed; Read and Close run on different goroutines
	hdrs [buffers]waveHdr
	bufs [buffers][bufBytes]byte
	cur  int
	rest []byte
}

func mmErr(name string, r uintptr) error {
	if r != 0 {
		return fmt.Errorf("%s failed: MMRESULT %d (no microphone, or microphone access is off in Privacy settings)", name, r)
	}
	return nil
}

// openMic records from the default microphone with the waveIn API.
func openMic(ctx context.Context) (io.ReadCloser, error) {
	w := &waveIn{}
	f := waveFormat{tag: 1, channels: 1, rate: 44100, byteRate: 88200, align: 2, bits: 16}
	var h uintptr
	r, _, _ := waveInOpen.Call(uintptr(unsafe.Pointer(&h)), waveMapper, uintptr(unsafe.Pointer(&f)), 0, 0, 0)
	if err := mmErr("waveInOpen", r); err != nil {
		return nil, err
	}
	w.h.Store(h)
	for i := range w.hdrs {
		w.hdrs[i] = waveHdr{data: &w.bufs[i][0], length: bufBytes}
		if err := w.queue(i, true); err != nil {
			w.Close()
			return nil, err
		}
	}
	r, _, _ = waveInStart.Call(h)
	if err := mmErr("waveInStart", r); err != nil {
		w.Close()
		return nil, err
	}
	go func() { <-ctx.Done(); w.Close() }()
	return w, nil
}

func (w *waveIn) queue(i int, prepare bool) error {
	size := unsafe.Sizeof(w.hdrs[i])
	if prepare {
		r, _, _ := waveInPrepare.Call(w.h.Load(), uintptr(unsafe.Pointer(&w.hdrs[i])), size)
		if err := mmErr("waveInPrepareHeader", r); err != nil {
			return err
		}
	}
	atomic.AndUint32(&w.hdrs[i].flags, ^uint32(whdrDone))
	r, _, _ := waveInAdd.Call(w.h.Load(), uintptr(unsafe.Pointer(&w.hdrs[i])), size)
	return mmErr("waveInAddBuffer", r)
}

// Read returns recorded bytes in order, waiting for the next buffer.
func (w *waveIn) Read(p []byte) (int, error) {
	for len(w.rest) == 0 {
		if w.h.Load() == 0 {
			return 0, io.EOF
		}
		h := &w.hdrs[w.cur]
		for atomic.LoadUint32(&h.flags)&whdrDone == 0 {
			time.Sleep(5 * time.Millisecond)
			if w.h.Load() == 0 {
				return 0, io.EOF
			}
		}
		w.rest = append(w.rest[:0], w.bufs[w.cur][:h.got]...)
		if err := w.queue(w.cur, false); err != nil {
			return 0, err
		}
		w.cur = (w.cur + 1) % buffers
	}
	n := copy(p, w.rest)
	w.rest = w.rest[n:]
	return n, nil
}

func (w *waveIn) Close() error {
	h := w.h.Swap(0)
	if h == 0 {
		return nil
	}
	waveInReset.Call(h)
	for i := range w.hdrs {
		waveInUnprep.Call(h, uintptr(unsafe.Pointer(&w.hdrs[i])), unsafe.Sizeof(w.hdrs[i]))
	}
	waveInClose.Call(h)
	return nil
}
