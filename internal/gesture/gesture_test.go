package gesture

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"coinchord/internal/dsp"
)

// synth builds frames: amp(i) shapes loudness per frame, hz(i) the pitch,
// noise mixes broadband friction (0 = pure ring, 1 = band noise).
func synth(n int, amp, hz func(i int) float64, noise float64) []dsp.Frame {
	r := rand.New(rand.NewPCG(1, 2))
	out := make([]dsp.Frame, n)
	for i := range n {
		buf := make([]int16, dsp.BlockSize)
		phase := 0.0
		for j := range buf {
			// narrow band noise: random phase jitter around the pitch
			phase += 2 * math.Pi * (hz(i) + noise*(r.Float64()-0.5)*3000) / dsp.SampleRate
			v := amp(i) * (math.Sin(phase)*(1-noise*0.5) + noise*0.3*(r.Float64()*2-1))
			buf[j] = int16(v * 32767)
		}
		out[i] = dsp.Analyze(buf)
	}
	return out
}

func flat(v float64) func(int) float64 { return func(int) float64 { return v } }

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func st(s string) Striker { return must(ParseStriker(s)) }

func g(s string) Gesture { return must(ParseGesture(s)) }

// kit builds a kit with synthetic prints at the given pitches.
func kit(pitch map[string]float64) Kit {
	var vs []Voice
	for _, code := range []string{"L", "R", "L1", "L2", "R1", "R2"} {
		if hz, ok := pitch[code]; ok {
			vs = append(vs, Voice{Striker: st(code), Name: code, Print: ToneAt(hz)})
		}
	}
	return must(NewKit(vs))
}

const L, R = 9200.0, 5800.0

var coins = kit(map[string]float64{"L": L, "R": R})

func classify(k Kit, fs []dsp.Frame) (Gesture, Metrics) {
	tu := DefaultTuning()
	m := Measure(fs, tu)
	got, _ := Classify(m, k, tu)
	return got, m
}

func TestClassifySynth(t *testing.T) {
	bump := func(k int) func(int) float64 {
		return func(i int) float64 { return 0.1 + 0.08*math.Abs(math.Sin(math.Pi*float64(i)/float64(k))) }
	}
	cases := []struct {
		name string
		fs   []dsp.Frame
		want string
	}{
		{"left tap", synth(3, flat(0.12), flat(L), 0), "L_TAP"},
		{"right tap", synth(3, flat(0.12), flat(R), 0), "R_TAP"},
		{"left knock", synth(3, flat(0.5), flat(L), 0), "L_KNOCK"},
		{"right line", synth(12, flat(0.1), flat(R), 1), "R_LINE"},
		{"left slide", synth(24, flat(0.1), flat(L), 1), "L_SLIDE"},
		{"left curve", synth(12, flat(0.1), func(i int) float64 { return L - 1500 + float64(i)*250 }, 1), "L_CURV"},
		{"right circle", synth(16, bump(8), flat(R), 1), "R_CIRC"},
		{"left zigzag", synth(16, bump(3), flat(L), 1), "L_ZIG"},
		{"both line", synth(12, flat(0.1), func(i int) float64 { return []float64{L, R}[i%2] }, 1), "L&R_LINE"},
	}
	for _, c := range cases {
		got, m := classify(coins, c.fs)
		if got != g(c.want) {
			t.Errorf("%s: got %v want %v (%+v)", c.name, got, c.want, m)
		}
	}
}

// A four-voice kit (two per hand) must tell every voice apart.
func TestClassifyFourVoices(t *testing.T) {
	pitch := map[string]float64{"L1": 12000, "L2": 7000, "R1": 9500, "R2": 4500}
	k := kit(pitch)
	for code, hz := range pitch {
		got, _ := classify(k, synth(3, flat(0.12), flat(hz), 0))
		if got != g(code+"_TAP") {
			t.Errorf("%s: got %v", code, got)
		}
	}
	if n := len(k.Sources()); n != 4+4 {
		t.Fatalf("sources %d want 4 solos + 4 cross-hand duos", n)
	}
}

// Negative control: a knob change must flip the result, so the test above
// reads the knobs it claims to test.
func TestClassifyKnobsMatter(t *testing.T) {
	tu := DefaultTuning()
	m := Measure(synth(3, flat(0.5), flat(L), 0), tu)
	if got, _ := Classify(m, coins, tu); got.Shape != Knock {
		t.Fatal("expected knock")
	}
	tu.KnockRatio = 5
	if got, _ := Classify(m, coins, tu); got.Shape != Tap {
		t.Fatalf("knock ratio 5: got %v want tap", got)
	}
}

func TestParseRejects(t *testing.T) {
	for _, bad := range []string{"L6", "X", "LL", "L&L1_TAP"} {
		if _, err := ParseGesture(bad + "_TAP"); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
	if _, err := NewKit([]Voice{{Striker: st("L")}, {Striker: st("L")}}); err == nil {
		t.Error("duplicate striker: want error")
	}
}

func TestSegmenterHangover(t *testing.T) {
	tu := DefaultTuning()
	loud, quiet := dsp.Frame{RMS: 1}, dsp.Frame{RMS: 0}
	var s Segmenter
	var strokes [][]dsp.Frame
	for _, f := range []dsp.Frame{loud, loud, quiet, loud, quiet, quiet, quiet, loud, quiet, quiet, quiet} {
		if st, ok := s.Push(f, tu); ok {
			strokes = append(strokes, st)
		}
	}
	if len(strokes) != 2 || len(strokes[0]) != 4 || len(strokes[1]) != 1 {
		t.Fatalf("got %d strokes: %v", len(strokes), strokes)
	}
}

func TestComposer(t *testing.T) {
	tu := DefaultTuning()
	ms := time.Millisecond
	a, b := g("L_TAP"), g("R_LINE")
	var c Composer
	if _, ok := c.Push(a, 0, tu); ok {
		t.Fatal("first gesture must wait")
	}
	if ch, ok := c.Push(b, 100*ms, tu); !ok || ch != Pair(a, b) {
		t.Fatalf("pair: %v %v", ch, ok)
	}
	c.Push(a, 1000*ms, tu)
	if _, ok := c.Tick(1200*ms, tu); ok {
		t.Fatal("tick inside window must wait")
	}
	if ch, ok := c.Tick(1400*ms, tu); !ok || ch != Single(a) {
		t.Fatalf("single: %v %v", ch, ok)
	}
}

func TestChordText(t *testing.T) {
	for _, c := range coins.Ease() {
		var p Chord
		if err := p.UnmarshalText([]byte(c.String())); err != nil || p != c {
			t.Fatalf("roundtrip %v: %v %v", c, p, err)
		}
	}
	for _, bad := range []string{"", "X_TAP", "L_FOO", "L_TAP+", "L_TAP+R_TAP+L_TAP", "L&L1_TAP"} {
		if _, err := ParseChord(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
	if n := len(coins.Ease()); n != 21+21*21 {
		t.Fatalf("ease count %d", n)
	}
}
