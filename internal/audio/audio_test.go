package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"sync/atomic"
	"testing"

	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
	"coinchord/internal/kits"
)

// pcm appends n frames of a sine at hz with amplitude amp (0 = silence).
func pcm(b *bytes.Buffer, n int, hz, amp float64) {
	for i := range n * dsp.BlockSize {
		v := amp * math.Sin(2*math.Pi*hz*float64(i)/dsp.SampleRate)
		_ = binary.Write(b, binary.LittleEndian, int16(v*32767))
	}
}

func listen(t *testing.T, raw *bytes.Buffer) []string {
	t.Helper()
	k, err := kits.Presets[0].Kit()
	if err != nil {
		t.Fatal(err)
	}
	var setup atomic.Pointer[Setup]
	setup.Store(&Setup{k, gesture.DefaultTuning()})
	var got []string
	Listen(raw, &setup, func(e any) {
		switch e := e.(type) {
		case Stroke:
			got = append(got, "stroke "+e.Gesture.String())
		case Chord:
			got = append(got, "chord "+e.Chord.String())
		}
	})
	return got
}

func TestListenChords(t *testing.T) {
	var raw bytes.Buffer
	pcm(&raw, 10, 0, 0)
	pcm(&raw, 3, 9200, 0.12) // left tap
	pcm(&raw, 5, 0, 0)       // 116 ms gap: inside the chord window
	pcm(&raw, 3, 5800, 0.12) // right tap
	pcm(&raw, 40, 0, 0)      // long silence
	pcm(&raw, 3, 5800, 0.12) // right tap alone
	pcm(&raw, 40, 0, 0)
	got := listen(t, &raw)
	want := []string{"stroke L_TAP", "stroke R_TAP", "chord L_TAP+R_TAP", "stroke R_TAP", "chord R_TAP"}
	if len(got) != len(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}
