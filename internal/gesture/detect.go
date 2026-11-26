package gesture

import (
	"math"
	"slices"
	"time"

	"coinchord/internal/dsp"
)

// Tuning holds the calibrated values and the detection knobs.
// Load it through Clamp so every knob stays inside its range.
type Tuning struct {
	Trigger        float64 `json:"trigger"`
	TapPeak        float64 `json:"tap_peak"`
	ScrapeFlat     float64 `json:"scrape_flat"`
	TapMaxMs       float64 `json:"tap_max_ms"`
	KnockRatio     float64 `json:"knock_ratio"`
	CurveDriftHz   float64 `json:"curve_drift_hz"`
	SlideMs        float64 `json:"slide_ms"`
	RippleRatio    float64 `json:"ripple_ratio"`
	CirclePeaks    float64 `json:"circle_peaks"`
	ZigzagPeaks    float64 `json:"zigzag_peaks"`
	ZigzagPeriodMs float64 `json:"zigzag_period_ms"`
	BothMargin     float64 `json:"both_margin"`
	HangFrames     float64 `json:"hang_frames"`
	ChordWinMs     float64 `json:"chord_win_ms"`
}

// Knob describes one adjustable value of Tuning.
type Knob struct {
	Name, Help     string
	Min, Max, Step float64
	Of             func(*Tuning) *float64
}

// Knobs lists every tunable value with its safe range.
var Knobs = []Knob{
	{"Trigger level", "Loudness that starts a stroke. Raise it if noise types words.", 0.001, 1, 0.002, func(t *Tuning) *float64 { return &t.Trigger }},
	{"Tap loudness", "Typical peak loudness of a normal tap.", 0.01, 1, 0.01, func(t *Tuning) *float64 { return &t.TapPeak }},
	{"Friction flatness", "Above this, a sound is a drag, not a tap.", 0.02, 0.9, 0.01, func(t *Tuning) *float64 { return &t.ScrapeFlat }},
	{"Tap max ms", "Strokes shorter than this can be taps.", 40, 300, 5, func(t *Tuning) *float64 { return &t.TapMaxMs }},
	{"Knock ratio", "A knock is this many times louder than a tap.", 1.2, 5, 0.1, func(t *Tuning) *float64 { return &t.KnockRatio }},
	{"Curve drift Hz", "Pitch change that makes a line a curve.", 100, 3000, 20, func(t *Tuning) *float64 { return &t.CurveDriftHz }},
	{"Slide ms", "Straight drags longer than this are slides.", 200, 1500, 10, func(t *Tuning) *float64 { return &t.SlideMs }},
	{"Ripple ratio", "How much a loudness bump must stand out to count.", 1.01, 2, 0.01, func(t *Tuning) *float64 { return &t.RippleRatio }},
	{"Circle bumps", "Loudness bumps needed for a circle.", 1, 8, 1, func(t *Tuning) *float64 { return &t.CirclePeaks }},
	{"Zigzag bumps", "Loudness bumps needed for a zigzag.", 2, 12, 1, func(t *Tuning) *float64 { return &t.ZigzagPeaks }},
	{"Zigzag max period ms", "Bumps closer than this make a zigzag.", 30, 300, 5, func(t *Tuning) *float64 { return &t.ZigzagPeriodMs }},
	{"Two-hand margin", "How even a sound mix must be to count as both hands.", 0.02, 0.45, 0.01, func(t *Tuning) *float64 { return &t.BothMargin }},
	{"Gap frames", "Short silences (frames) allowed inside one stroke.", 0, 8, 1, func(t *Tuning) *float64 { return &t.HangFrames }},
	{"Chord window ms", "Two gestures closer than this make one chord.", 120, 1200, 10, func(t *Tuning) *float64 { return &t.ChordWinMs }},
}

// DefaultTuning is a starting point before calibration.
func DefaultTuning() Tuning {
	return Tuning{
		Trigger: 0.032, TapPeak: 0.12, ScrapeFlat: 0.26,
		TapMaxMs: 125, KnockRatio: 1.8, CurveDriftHz: 480, SlideMs: 450, RippleRatio: 1.15,
		CirclePeaks: 2, ZigzagPeaks: 4, ZigzagPeriodMs: 90, BothMargin: 0.15, HangFrames: 2, ChordWinMs: 360,
	}
}

// Clamp returns t with every knob forced inside its range.
func (t Tuning) Clamp() Tuning {
	for _, k := range Knobs {
		p := k.Of(&t)
		*p = math.Min(k.Max, math.Max(k.Min, *p))
	}
	return t
}

// Metrics are the measured properties of one stroke.
type Metrics struct {
	DurMs, Peak, Centroid float64
	Flat                  float64 // lowest spectral flatness of any frame: low = rings, high = scrapes
	Print                 Print   // unit band energies of the whole stroke
	DriftHz               float64
	Peaks                 int
	PeriodMs              float64 // mean spacing of loudness bumps
}

// Measure computes stroke metrics from its frames. fs must not be empty.
func Measure(fs []dsp.Frame, t Tuning) Metrics {
	n := len(fs)
	m := Metrics{DurMs: float64(n) * dsp.FrameMs, Flat: 1}
	cents := make([]float64, n)
	for i, f := range fs {
		m.Peak = math.Max(m.Peak, f.RMS)
		// the most tonal frame: a tap rings at least once (its click frame is
		// broadband), while a drag is friction noise in every frame
		m.Flat = math.Min(m.Flat, f.Flat)
		cents[i] = f.Cent
		m.Print.addFrame(f)
	}
	m.Print = m.Print.unit()
	sorted := slices.Clone(cents)
	slices.Sort(sorted)
	m.Centroid = sorted[n/2]
	q := max(1, n/4)
	var a, b float64
	for i := range q {
		a += cents[i]
		b += cents[n-1-i]
	}
	m.DriftHz = math.Abs(b-a) / float64(q)
	// Hysteresis bump count: a bump rises RippleRatio above the last valley
	// and falls RippleRatio below its top.
	first, last := -1, -1
	lo, hi, hiAt, rising := fs[0].RMS, fs[0].RMS, 0, true
	for i, f := range fs {
		x := f.RMS
		switch {
		case rising && x > hi:
			hi, hiAt = x, i
		case rising && x*t.RippleRatio < hi && hi > lo*t.RippleRatio:
			m.Peaks++
			last = hiAt
			if first < 0 {
				first = hiAt
			}
			rising, lo = false, x
		case !rising && x < lo:
			lo = x
		case !rising && x > lo*t.RippleRatio:
			rising, hi, hiAt = true, x, i
		}
	}
	if m.Peaks >= 2 {
		m.PeriodMs = float64(last-first) / float64(m.Peaks-1) * dsp.FrameMs
	}
	return m
}

// Classify maps stroke metrics to a gesture.
func Classify(m Metrics, k Kit, t Tuning) (Gesture, Match) {
	src, why := k.match(m.Print, t.BothMargin)
	return Gesture{src, shapeOf(m, t)}, why
}

func shapeOf(m Metrics, t Tuning) Shape {
	switch {
	case m.DurMs <= t.TapMaxMs && m.Flat <= t.ScrapeFlat && m.Peak >= t.KnockRatio*t.TapPeak:
		return Knock
	case m.DurMs <= t.TapMaxMs && m.Flat <= t.ScrapeFlat:
		return Tap
	case float64(m.Peaks) >= t.ZigzagPeaks && m.PeriodMs <= t.ZigzagPeriodMs:
		return Zigzag
	case float64(m.Peaks) >= t.CirclePeaks:
		return Circle
	case m.DriftHz >= t.CurveDriftHz:
		return Curve
	case m.DurMs >= t.SlideMs:
		return Slide
	}
	return Line
}

// Segmenter cuts a frame stream into strokes. Short silences inside
// a stroke (up to HangFrames) stay part of it.
type Segmenter struct {
	cur, tail []dsp.Frame
}

// Push adds a frame. It returns a finished stroke when one ends.
func (s *Segmenter) Push(f dsp.Frame, t Tuning) ([]dsp.Frame, bool) {
	switch {
	case f.RMS > t.Trigger:
		s.cur = append(append(s.cur, s.tail...), f)
		s.tail = nil
	case len(s.cur) > 0 && len(s.tail) < int(t.HangFrames):
		s.tail = append(s.tail, f)
	case len(s.cur) > 0:
		out := s.cur
		s.cur, s.tail = nil, nil
		return out, true
	}
	return nil, false
}

// Active reports whether a stroke is in progress.
func (s *Segmenter) Active() bool { return len(s.cur) > 0 }

// Composer fuses gestures inside the chord window into chords.
type Composer struct {
	first Gesture
	at    time.Duration
	has   bool
}

// Push adds a gesture at time now. It returns a chord when one is complete.
func (c *Composer) Push(g Gesture, now time.Duration, t Tuning) (Chord, bool) {
	if !c.has {
		c.first, c.at, c.has = g, now, true
		return Chord{}, false
	}
	if now-c.at <= win(t) {
		c.has = false
		return Pair(c.first, g), true
	}
	prev := c.first
	c.first, c.at = g, now
	return Single(prev), true
}

// Tick flushes a waiting single gesture when the window has passed.
func (c *Composer) Tick(now time.Duration, t Tuning) (Chord, bool) {
	if c.has && now-c.at > win(t) {
		c.has = false
		return Single(c.first), true
	}
	return Chord{}, false
}

func win(t Tuning) time.Duration { return time.Duration(t.ChordWinMs * float64(time.Millisecond)) }
