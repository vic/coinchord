// Package audio reads the microphone (or a raw file) and emits detection events.
package audio

import (
	"context"
	"io"
	"os"
	"sync/atomic"
	"time"

	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
)

// Open starts the microphone, or opens path when it is not empty.
// The input is raw signed 16-bit little-endian mono at 44100 Hz.
func Open(ctx context.Context, path string) (io.ReadCloser, error) {
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return &paced{f, time.Now(), 0}, nil
	}
	return openMic(ctx)
}

// Setup is what the listener needs to classify; swap it at any time.
type Setup struct {
	Kit    gesture.Kit
	Tuning gesture.Tuning
}

// Frame is one analyzed block.
type Frame struct {
	dsp.Frame
	At time.Duration
}

// Stroke is one finished stroke with its classification.
type Stroke struct {
	Metrics gesture.Metrics
	Gesture gesture.Gesture
	Match   gesture.Match
	At      time.Duration
}

// Chord is a finished chord.
type Chord struct {
	Chord gesture.Chord
	At    time.Duration
}

// Stopped reports the end of the input.
type Stopped struct{ Err error }

// captureBuffer is how many raw blocks may wait for analysis (~1.5 s).
const captureBuffer = 64

// capture reads raw blocks on its own goroutine, so a slow consumer never
// makes the sound card overrun. The channel closes at the end of the input;
// the error is sent on errc first.
func capture(r io.Reader, errc chan<- error) <-chan []byte {
	blocks := make(chan []byte, captureBuffer)
	go func() {
		defer close(blocks)
		for {
			raw := make([]byte, dsp.BlockSize*2)
			if _, err := io.ReadFull(r, raw); err != nil {
				errc <- err
				return
			}
			blocks <- raw
		}
	}()
	return blocks
}

// Listen analyzes audio from r until it ends and emits Frame, Stroke, Chord
// and finally Stopped. Capture runs on its own goroutine; analysis runs on
// the caller's goroutine, so run Listen with go. Time is counted in frames,
// so file input replays exactly like live input.
func Listen(r io.Reader, setup *atomic.Pointer[Setup], emit func(any)) {
	errc := make(chan error, 1)
	var seg gesture.Segmenter
	var comp gesture.Composer
	i := 0
	for raw := range capture(r, errc) {
		f := dsp.Decode(raw)
		s := setup.Load()
		at := time.Duration(float64(i) * dsp.FrameMs * float64(time.Millisecond))
		i++
		emit(Frame{f, at})
		if fs, ok := seg.Push(f, s.Tuning); ok {
			m := gesture.Measure(fs, s.Tuning)
			g, why := gesture.Classify(m, s.Kit, s.Tuning)
			emit(Stroke{m, g, why, at})
			if c, ok := comp.Push(g, at, s.Tuning); ok {
				emit(Chord{c, at})
			}
		}
		// a stroke in progress may become the second gesture: wait for it
		if seg.Active() {
			continue
		}
		if c, ok := comp.Tick(at, s.Tuning); ok {
			emit(Chord{c, at})
		}
	}
	emit(Stopped{<-errc})
}

// paced plays a file at the speed a microphone would deliver it.
type paced struct {
	f     *os.File
	start time.Time
	n     int64
}

const bytesPerSecond = 44100 * 2

func (p *paced) Read(b []byte) (int, error) {
	n, err := p.f.Read(b)
	p.n += int64(n)
	time.Sleep(time.Until(p.start.Add(time.Duration(p.n) * time.Second / bytesPerSecond)))
	return n, err
}

func (p *paced) Close() error { return p.f.Close() }
