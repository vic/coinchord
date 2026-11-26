package app

import (
	"fmt"
	"math"
	"slices"

	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/gesture"
)

type calStep uint8

const (
	calStart calStep = iota
	calQuiet
	calVoice
	calDrag
	calTest
	calReport
	calSteps
)

var calStepNames = [calSteps]string{"start", "quiet", "voice", "drag", "test", "report"}

const (
	quietFrames  = 86 // ~2 s
	tapsNeeded   = 5
	dragsNeeded  = 3
	triesEach    = 3
	sameSound    = 0.75 // taps of one striker less alike than this are retried
	reliableRate = 2.0 / 3
)

type cal struct {
	step    calStep
	voice   int
	quiet   audio.Monitor
	taps    [][]gesture.Metrics
	drags   []gesture.Metrics
	target  int
	msg     string
	good    bool
	attempt []bool
}

func (c *cal) say(good bool, f string, a ...any) { c.good, c.msg = good, fmt.Sprintf(f, a...) }

func (c *cal) enter(a *App, s calStep) {
	c.step, c.msg, c.attempt = s, "", nil
	switch s {
	case calQuiet:
		c.quiet = audio.Monitor{}
	case calVoice:
		c.taps, c.voice = make([][]gesture.Metrics, len(a.prof.Kit.Voices())), 0
	case calDrag:
		c.drags = nil
	case calTest:
		c.target = -1
		c.next(a)
	}
	a.dirty = true
}

func (c *cal) onFrame(a *App, f audio.Frame) {
	if a.page != pageCalibrate || c.step != calQuiet {
		return
	}
	c.quiet.Add(f.Frame)
	if c.quiet.Frames() < quietFrames {
		if c.quiet.Frames()%8 == 0 {
			a.dirty = true // progress bar
		}
		return
	}
	a.prof.Tuning.Trigger = audio.TriggerFor(c.quiet)
	a.prof.Tuning = a.prof.Tuning.Clamp()
	a.save()
	c.enter(a, calVoice)
	c.say(true, "Room measured. Now let's learn your strikers.")
}

func (c *cal) onStroke(a *App, s audio.Stroke) {
	if a.page != pageCalibrate {
		return
	}
	if want, ok := c.wanted(a); ok && a.opt.Labels != nil {
		b := mustJSON(map[string]any{"ms": s.At.Milliseconds(), "want": want.String(), "got": s.Gesture.String()})
		if _, err := a.opt.Labels.Write(append(b, '\n')); err != nil {
			a.notice = "Could not write labels: " + err.Error()
		}
	}
	switch c.step {
	case calVoice:
		c.onTap(a, s.Metrics)
	case calDrag:
		c.onDrag(a, s.Metrics)
	case calTest:
		c.onTest(a, s.Gesture)
	}
}

func avgPrint(ms []gesture.Metrics) gesture.Print {
	ps := make([]gesture.Print, len(ms))
	for i, x := range ms {
		ps[i] = x.Print
	}
	return gesture.Average(ps)
}

func median(xs []float64) float64 {
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[len(s)/2]
}

func (c *cal) onTap(a *App, mt gesture.Metrics) {
	vs := a.prof.Kit.Voices()
	if c.voice >= len(vs) || len(c.taps) != len(vs) {
		return
	}
	if mt.DurMs > a.prof.Tuning.TapMaxMs*2 {
		c.say(false, "That sounded like a drag. Just a quick tap, then lift.")
		return
	}
	got := c.taps[c.voice]
	if len(got) > 0 {
		if sim := gesture.Similarity(avgPrint(got), mt.Print); sim < sameSound {
			c.say(false, "That one sounded different (%.0f%% alike). Same grip, same spot — try again.", sim*100)
			return
		}
		c.say(true, "Nice and consistent!")
	} else {
		c.say(true, "Got it. Keep going.")
	}
	c.taps[c.voice] = append(got, mt)
	if len(c.taps[c.voice]) < tapsNeeded {
		return
	}
	a.prof.Kit = a.prof.Kit.WithPrint(c.voice, avgPrint(c.taps[c.voice]))
	a.save()
	c.say(true, "%s learned.", vs[c.voice].Name)
	c.voice++
	if c.voice == len(vs) {
		c.step = calDrag
	}
}

func (c *cal) onDrag(a *App, mt gesture.Metrics) {
	if mt.DurMs <= a.prof.Tuning.TapMaxMs {
		c.say(false, "That was a tap. Slide the object across the desk instead, about an inch.")
		return
	}
	c.drags = append(c.drags, mt)
	c.say(true, "Nice drag.")
	if len(c.drags) < dragsNeeded {
		return
	}
	var tapFlat, tapPeak, dragFlat []float64
	for _, ts := range c.taps {
		for _, t := range ts {
			tapFlat, tapPeak = append(tapFlat, t.Flat), append(tapPeak, t.Peak)
		}
	}
	for _, d := range c.drags {
		dragFlat = append(dragFlat, d.Flat)
	}
	if len(tapFlat) > 0 {
		a.prof.Tuning.ScrapeFlat = (median(tapFlat) + median(dragFlat)) / 2
		a.prof.Tuning.TapPeak = median(tapPeak)
	}
	a.prof.Tuning = a.prof.Tuning.Clamp()
	a.prof.Calibrated = true
	a.save()
	c.enter(a, calTest)
	c.say(true, "Calibrated! Now let's see which gestures the mic hears clearly.")
}

func ratio(s config.Score) float64 { return float64(s.Hits) / math.Max(1, float64(s.Tries)) }

func (c *cal) onTest(a *App, g gesture.Gesture) {
	gs := a.prof.Kit.Gestures()
	if c.target < 0 || c.target >= len(gs) {
		return
	}
	want := gs[c.target]
	s := a.prof.Quality[want.String()]
	s.Tries++
	hit := g == want
	if hit {
		s.Hits++
		c.say(true, "Yes! Clean %s.", want)
	} else {
		c.say(false, "I heard %s. %s", g, hint(want, g))
	}
	a.prof.Quality[want.String()] = s
	c.attempt = append(c.attempt, hit)
	a.save()
	if len(c.attempt) >= triesEach {
		c.next(a)
	}
}

// next moves to the next gesture with fewer than triesEach tries, or to the report.
func (c *cal) next(a *App) {
	gs := a.prof.Kit.Gestures()
	c.attempt = nil
	for i := 1; i <= len(gs); i++ {
		j := (c.target + i + len(gs)) % len(gs)
		if a.prof.Quality[gs[j].String()].Tries < triesEach {
			c.target = j
			return
		}
	}
	c.step = calReport
}

func (c *cal) retestWeak(a *App) {
	for _, g := range a.prof.Kit.Gestures() {
		if s := a.prof.Quality[g.String()]; s.Tries > 0 && ratio(s) < reliableRate {
			delete(a.prof.Quality, g.String())
		}
	}
	a.save()
	c.enter(a, calTest)
}

func hint(want, got gesture.Gesture) string {
	if want.Src != got.Src {
		return fmt.Sprintf("It sounded like %s. Use only %s.", got.Src, want.Src)
	}
	type pair struct{ w, g gesture.Shape }
	h := map[pair]string{
		{gesture.Tap, gesture.Knock}:     "A little softer.",
		{gesture.Knock, gesture.Tap}:     "A bit firmer, like knocking on a door.",
		{gesture.Curve, gesture.Line}:    "Hook your wrist more at the end, like a J.",
		{gesture.Line, gesture.Curve}:    "Keep the drag straight, wrist still.",
		{gesture.Slide, gesture.Line}:    "Drag longer and slower.",
		{gesture.Line, gesture.Slide}:    "A shorter drag, about an inch.",
		{gesture.Zigzag, gesture.Circle}: "Faster, shorter zigs back and forth.",
		{gesture.Circle, gesture.Zigzag}: "Smooth round turns, not zigs.",
	}
	if s, ok := h[pair{want.Shape, got.Shape}]; ok {
		return s
	}
	return want.Shape.HowTo()
}

// wanted is the gesture the current step asks for.
func (c *cal) wanted(a *App) (gesture.Gesture, bool) {
	vs := a.prof.Kit.Voices()
	gs := a.prof.Kit.Gestures()
	switch {
	case c.step == calVoice && c.voice < len(vs):
		return gesture.Gesture{Src: gesture.Solo(vs[c.voice].Striker), Shape: gesture.Tap}, true
	case c.step == calDrag && len(vs) > 0:
		return gesture.Gesture{Src: gesture.Solo(vs[0].Striker), Shape: gesture.Line}, true
	case c.step == calTest && c.target >= 0 && c.target < len(gs):
		return gs[c.target], true
	}
	return gesture.Gesture{}, false
}
