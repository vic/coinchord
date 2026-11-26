// Package audio reads the microphone (or a raw file) and emits detection events.
package audio

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"time"

	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
)

// Open starts the microphone, or opens path when it is not empty.
// The input is raw signed 16-bit little-endian mono at 44100 Hz.
func Open(ctx context.Context, path string) (io.ReadCloser, error) {
	if path != "" {
		return os.Open(path)
	}
	cmd := exec.CommandContext(ctx, "arecord", "-q", "-f", "S16_LE", "-r", "44100", "-c", "1", "-t", "raw")
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(ctx, "rec", "-q", "-t", "raw", "-r", "44100", "-e", "signed-integer", "-b", "16", "-c", "1", "-")
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return out, cmd.Start()
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

// Listen reads frames from r until it ends and emits Frame, Stroke, Chord
// and finally Stopped. Time is counted in frames, so file input replays
// exactly like live input.
func Listen(r io.Reader, setup *atomic.Pointer[Setup], emit func(any)) {
	var seg gesture.Segmenter
	var comp gesture.Composer
	for i := 0; ; i++ {
		f, err := dsp.Read(r)
		if err != nil {
			emit(Stopped{err})
			return
		}
		s := setup.Load()
		at := time.Duration(float64(i) * dsp.FrameMs * float64(time.Millisecond))
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
}
