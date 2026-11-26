package audio

import (
	"testing"

	"coinchord/internal/dsp"
)

func feed(n int, rms, peak float64) Monitor {
	var m Monitor
	for range n {
		m.Add(dsp.Frame{RMS: rms, Peak: peak})
	}
	return m
}

// Levels measured on a real laptop mic: muted = zeros, 100% gain = rms 0.79,
// 10% gain = rms 0.0017 room noise.
func TestMicState(t *testing.T) {
	cases := []struct {
		name string
		m    Monitor
		want MicState
	}{
		{"none", Monitor{}, MicWaiting},
		{"muted", feed(60, 0, 0), MicSilent},
		{"short zeros", feed(10, 0, 0), MicLive},
		{"saturated", feed(60, 0.79, 0.99997), MicClipping},
		{"loud noise", feed(60, 0.3, 0.9), MicClipping},
		{"room", feed(60, 0.0017, 0.01), MicLive},
	}
	for _, c := range cases {
		if got := c.m.State(); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
	m := feed(100, 0.002, 0.01)
	for range 20 {
		m.Add(dsp.Frame{RMS: 0.5, Peak: 0.6}) // strokes must not raise the noise floor
	}
	if n := m.Noise(); n != 0.002 {
		t.Fatalf("noise %v", n)
	}
}

func TestParseLevel(t *testing.T) {
	l, err := parseLevel("Volume: 0.40 [MUTED]\n")
	if err != nil || l.Volume != 0.4 || !l.Muted {
		t.Fatalf("%+v %v", l, err)
	}
	if _, err := parseLevel("garbage"); err == nil {
		t.Fatal("want error")
	}
}
