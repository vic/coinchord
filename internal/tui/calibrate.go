package tui

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/gesture"
)

type calStep uint8

const (
	calStart calStep = iota
	calQuiet
	calVoice
	calScrape
	calTest
	calReport
	calSteps
)

var calStepNames = [calSteps]string{"Start", "Quiet room", "Strikers", "Drag", "Gesture test", "Report"}

const (
	quietFrames = 86 // ~2 s
	tapsNeeded  = 5
	dragsNeeded = 3
	triesEach   = 3
)

type calAct uint8

const (
	calGo     calAct = iota // go to step i
	calRedo                 // redo current striker or step
	calSkip                 // skip current test gesture
	calTarget               // test gesture i
	calRetestWeak
)

type calMsg struct {
	act calAct
	i   int
}

type calState struct {
	step    calStep
	voice   int
	quiet   []float64
	taps    [][]gesture.Metrics
	drags   []gesture.Metrics
	target  int
	msg     string
	good    bool
	attempt []bool // results for the current test target, newest last
}

func (c *calState) say(good bool, f string, a ...any) { c.good, c.msg = good, fmt.Sprintf(f, a...) }

func (c *calState) on(m *Model, msg calMsg) {
	switch msg.act {
	case calGo:
		c.enter(m, calStep(msg.i))
	case calRedo:
		c.enter(m, c.step)
	case calSkip:
		c.next(m)
	case calTarget:
		c.target, c.attempt, c.msg = msg.i, nil, ""
	case calRetestWeak:
		for _, g := range m.prof.Kit.Gestures() {
			if s := m.prof.Quality[g.String()]; s.Tries > 0 && ratio(s) < 2.0/3 {
				delete(m.prof.Quality, g.String())
			}
		}
		m.save()
		c.enter(m, calTest)
		c.next(m)
	}
}

func (c *calState) enter(m *Model, s calStep) {
	c.step, c.msg, c.attempt = s, "", nil
	switch s {
	case calQuiet:
		c.quiet = nil
	case calVoice:
		c.taps = make([][]gesture.Metrics, len(m.prof.Kit.Voices()))
		c.voice = 0
	case calScrape:
		c.drags = nil
	case calTest:
		c.target = 0
	}
}

func (c *calState) onFrame(m *Model, f audio.Frame) {
	if m.tab != tabCalibrate || c.step != calQuiet {
		return
	}
	c.quiet = append(c.quiet, f.RMS)
	if len(c.quiet) < quietFrames {
		return
	}
	q := slices.Clone(c.quiet)
	sort.Float64s(q)
	m.prof.Tuning.Trigger = math.Max(q[len(q)*9/10]*3.8, 0.006)
	m.prof.Tuning = m.prof.Tuning.Clamp()
	m.save()
	c.enter(m, calVoice)
	c.say(true, "Room measured. Trigger set to %.3f.", m.prof.Tuning.Trigger)
}

func (c *calState) onStroke(m *Model, s audio.Stroke) {
	if m.tab != tabCalibrate {
		return
	}
	switch c.step {
	case calVoice:
		c.onTap(m, s.Metrics)
	case calScrape:
		c.onDrag(m, s.Metrics)
	case calTest:
		c.onTest(m, s.Gesture)
	}
}

func (c *calState) onTap(m *Model, mt gesture.Metrics) {
	vs := m.prof.Kit.Voices()
	if c.voice >= len(vs) {
		return
	}
	if mt.DurMs > m.prof.Tuning.TapMaxMs*2 {
		c.say(false, "That sounded like a long drag. Just a quick tap, then lift.")
		return
	}
	got := c.taps[c.voice]
	if len(got) > 0 {
		sim := gesture.Similarity(avgPrint(got), mt.Print)
		if sim < 0.75 {
			c.say(false, "That tap sounded different (%s alike). Keep the same grip and spot. Try again.", pct(sim))
			return
		}
		c.say(true, "Consistent! %s alike to your other taps.", pct(sim))
	} else {
		c.say(true, "Got it. Keep going.")
	}
	c.taps[c.voice] = append(got, mt)
	if len(c.taps[c.voice]) < tapsNeeded {
		return
	}
	m.prof.Kit = m.prof.Kit.WithPrint(c.voice, avgPrint(c.taps[c.voice]))
	m.save()
	c.say(true, "%s learned.", vs[c.voice].Name)
	c.voice++
	if c.voice == len(vs) {
		c.step = calScrape
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
	sort.Float64s(s)
	return s[len(s)/2]
}

func (c *calState) onDrag(m *Model, mt gesture.Metrics) {
	if mt.DurMs <= m.prof.Tuning.TapMaxMs {
		c.say(false, "That was a tap. Drag the object across the desk instead, about 1 inch.")
		return
	}
	c.drags = append(c.drags, mt)
	c.say(true, "Nice drag (%.0f ms).", mt.DurMs)
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
		m.prof.Tuning.ScrapeFlat = (median(tapFlat) + median(dragFlat)) / 2
		m.prof.Tuning.TapPeak = median(tapPeak)
	}
	m.prof.Tuning = m.prof.Tuning.Clamp()
	m.prof.Calibrated = true
	m.save()
	c.enter(m, calTest)
	c.next(m)
	c.say(true, "Calibrated! Now let's see which gestures the mic hears clearly.")
}

func ratio(s config.Score) float64 { return float64(s.Hits) / math.Max(1, float64(s.Tries)) }

func (c *calState) onTest(m *Model, g gesture.Gesture) {
	gs := m.prof.Kit.Gestures()
	if c.target >= len(gs) {
		return
	}
	want := gs[c.target]
	s := m.prof.Quality[want.String()]
	s.Tries++
	hit := g == want
	if hit {
		s.Hits++
		c.say(true, praise[s.Tries%len(praise)], chip(want))
	} else {
		c.say(false, "Close! I heard %s. %s", chip(g), hint(want, g))
	}
	m.prof.Quality[want.String()] = s
	c.attempt = append(c.attempt, hit)
	m.save()
	if len(c.attempt) >= triesEach {
		c.next(m)
	}
}

// next moves the test to the next gesture with fewer than triesEach tries.
func (c *calState) next(m *Model) {
	gs := m.prof.Kit.Gestures()
	c.attempt = nil
	for i := 1; i <= len(gs); i++ {
		j := (c.target + i) % len(gs)
		if m.prof.Quality[gs[j].String()].Tries < triesEach {
			c.target = j
			return
		}
	}
	c.step = calReport
}

var praise = []string{"Yes! Clean %s.", "Perfect %s. The mic heard it clearly.", "Nice! %s detected."}

func hint(want, got gesture.Gesture) string {
	if want.Src != got.Src {
		return fmt.Sprintf("The sound matched %s, not %s. Use only the %s striker(s).", got.Src, want.Src, want.Src)
	}
	type pair struct{ w, g gesture.Shape }
	h := map[pair]string{
		{gesture.Tap, gesture.Knock}:     "A little softer.",
		{gesture.Knock, gesture.Tap}:     "A bit firmer, like a knock on a door.",
		{gesture.Curve, gesture.Line}:    "Hook the wrist more at the end, like a J.",
		{gesture.Line, gesture.Curve}:    "Keep the drag straight, wrist still.",
		{gesture.Slide, gesture.Line}:    "Drag longer and slower.",
		{gesture.Line, gesture.Slide}:    "A shorter drag, about 1 inch.",
		{gesture.Zigzag, gesture.Circle}: "Faster, shorter zigs back and forth.",
		{gesture.Circle, gesture.Zigzag}: "Smooth round turns, not zigs.",
	}
	if s, ok := h[pair{want.Shape, got.Shape}]; ok {
		return s
	}
	switch want.Shape {
	case gesture.Tap, gesture.Knock:
		return "Lift right away after the hit."
	case gesture.Circle:
		return "Make 1.5 full turns, a little wider."
	}
	return want.Shape.HowTo()
}

func (c calState) view(m Model, w int) box {
	var steps []box
	for i, n := range calStepNames {
		st := lipgloss.NewStyle().Padding(0, 1).Foreground(cDim)
		mark := "○"
		switch {
		case calStep(i) == c.step:
			st, mark = st.Foreground(lipgloss.Color("#000000")).Background(cAccent).Bold(true), "◉"
		case calStep(i) < c.step:
			st, mark = st.Foreground(cOK), "✔"
		}
		steps = append(steps, click(text(st.Render(mark+" "+n)), calMsg{calGo, i}))
	}
	msg := text("")
	if c.msg != "" {
		st := sWarn
		if c.good {
			st = sOK
		}
		msg = text(st.Render(c.msg))
	}
	var body box
	switch c.step {
	case calStart:
		body = c.viewStart(m)
	case calQuiet:
		body = c.viewQuiet(m)
	case calVoice:
		body = c.viewVoice(m)
	case calScrape:
		body = c.viewScrape(m)
	case calTest:
		body = c.viewTest(m, w)
	default:
		body = c.viewReport(m, w)
	}
	return vjoin(hjoin(1, steps...), msg, wrap(sPanel.Width(w-2), body))
}

func (c calState) viewStart(m Model) box {
	var vs []string
	for _, v := range m.prof.Kit.Voices() {
		vs = append(vs, lipgloss.NewStyle().Foreground(handColor(v.Striker.Hand())).Render(fmt.Sprintf("  %-3s %s (%s hand)", v.Striker, v.Name, v.Striker.Hand().Word())))
	}
	return vjoin(
		text(sTitle.Render("Calibrate: teach the mic your kit")+"   "+sBold.Render(m.prof.Preset)),
		text(strings.Join(vs, "\n")),
		text(""),
		text("It takes about two minutes. Relax: there is no wrong answer here.\n"+
			" 1. Stay quiet 2 seconds, so CoinChord hears your room.\n"+
			" 2. Tap each striker 5 times, so it learns each sound.\n"+
			" 3. Drag one striker 3 times, so it learns friction.\n"+
			" 4. Try every gesture. You see live which ones the mic hears well.\n"+
			sDim.Render("Hold each object near its edge, palms resting on the desk.")),
		text(""),
		hjoin(2, hotButton("Start →", calMsg{calGo, int(calQuiet)}), button("Change kit", tabMsg(tabOverview)),
			button("Skip to gesture test", calMsg{calGo, int(calTest)})),
	)
}

func (c calState) viewQuiet(m Model) box {
	return vjoin(
		text(sTitle.Render("Shhh… stay still for 2 seconds")),
		text("CoinChord listens to fans, AC and street noise, and sets the trigger above them."),
		text(""),
		text(bar(float64(len(c.quiet)), quietFrames, 40, cAccent, -1)+sDim.Render(fmt.Sprintf("  %d%%", len(c.quiet)*100/quietFrames))),
		text(sDim.Render("room loudness ")+level(m.frame.RMS, m.prof.Tuning.Trigger)),
		text(""),
		button("Restart", calMsg{calRedo, 0}),
	)
}

func dots(n, of int, c lipgloss.Style) string {
	return c.Render(strings.Repeat("● ", n)) + sDim.Render(strings.Repeat("○ ", max(0, of-n)))
}

func (c calState) viewVoice(m Model) box {
	vs := m.prof.Kit.Voices()
	if c.voice >= len(vs) || len(c.taps) != len(vs) {
		return button("Start strikers", calMsg{calGo, int(calVoice)})
	}
	v := vs[c.voice]
	col := lipgloss.NewStyle().Foreground(handColor(v.Striker.Hand())).Bold(true)
	left := vjoin(
		text(sTitle.Render(fmt.Sprintf("Striker %d of %d", c.voice+1, len(vs)))),
		text("Tap your "+col.Render(v.Name)+" ("+v.Striker.Hand().Word()+" hand) "+sBold.Render("5 times")+"."),
		text(sDim.Render("Relaxed, natural volume. Lift after each tap.")),
		text(""),
		text(art(gesture.Tap, handColor(v.Striker.Hand()))),
		text(dots(len(c.taps[c.voice]), tapsNeeded, col)),
		text(""),
		hjoin(1, button("Redo strikers", calMsg{calRedo, 0}), button("Back", calMsg{calGo, int(calQuiet)})),
	)
	right := vjoin(
		text(sDim.Render("live sound (each letter marks a learned striker)")),
		text(spectrum(m.frame, m.prof.Kit, m.prof.Tuning.Trigger, 5)),
		text(""),
		text(separation(m, c.voice)),
	)
	return hjoin(4, left, right)
}

// separation shows how alike the learned strikers sound; alike = confusable.
func separation(m Model, learned int) string {
	vs := m.prof.Kit.Voices()
	if learned < 2 {
		return sDim.Render("After two strikers you will see how different they sound.")
	}
	lines := []string{sBold.Render("How alike they sound (lower is better)")}
	for i := 0; i < learned; i++ {
		for j := i + 1; j < learned; j++ {
			s := gesture.Similarity(vs[i].Print, vs[j].Print)
			grade := sOK.Render("great")
			switch {
			case s > 0.9:
				grade = sBad.Render("too alike: try other objects")
			case s > 0.75:
				grade = sWarn.Render("fair")
			case s > 0.5:
				grade = sOK.Render("good")
			}
			lines = append(lines, fmt.Sprintf("%-3s vs %-3s %s %s %s", vs[i].Striker, vs[j].Striker, bar(s, 1, 12, cAccent, -1), pct(s), grade))
		}
	}
	return strings.Join(lines, "\n")
}

func (c calState) viewScrape(m Model) box {
	v := m.prof.Kit.Voices()[0]
	return vjoin(
		text(sTitle.Render("Drag 3 times")),
		text("Drag your "+sBold.Render(v.Name)+" in a straight line, about 1 inch, 3 times."),
		text(sDim.Render("This teaches the difference between a tap and friction.")),
		text(""),
		text(art(gesture.Line, handColor(v.Striker.Hand()))),
		text(dots(len(c.drags), dragsNeeded, sOK)),
		text(""),
		text(separation(m, len(m.prof.Kit.Voices()))),
		button("Redo", calMsg{calRedo, 0}),
	)
}

func (c calState) viewTest(m Model, w int) box {
	gs := m.prof.Kit.Gestures()
	if c.target >= len(gs) {
		return text("")
	}
	want := gs[c.target]
	var res []string
	for _, ok := range c.attempt {
		res = append(res, map[bool]string{true: sOK.Render("✔"), false: sBad.Render("✘")}[ok])
	}
	res = append(res, strings.Repeat(sDim.Render("·"), triesEach-len(c.attempt)))
	cur := vjoin(
		text(sTitle.Render("Try this gesture")+sDim.Render(fmt.Sprintf("  (%d of %d)", c.target+1, len(gs)))),
		text(""),
		hjoin(3, text(art(want.Shape, srcColor(want.Src))), vjoin(
			text(chip(want)),
			text(sourceWords(m, want.Src)),
			text(lipgloss.NewStyle().Width(40).Render(want.Shape.HowTo())),
			text("tries "+strings.Join(res, " ")),
		)),
		text(""),
		hjoin(1, button("Skip", calMsg{calSkip, 0}), button("Report →", calMsg{calGo, int(calReport)})),
		text(sDim.Render("This measures the mic, not you. Skip anything; come back any time.")),
	)
	var cards []box
	for i, g := range gs {
		s := m.prof.Quality[g.String()]
		score := sDim.Render(" -- ")
		if s.Tries > 0 {
			score = pct(ratio(s))
		}
		st := lipgloss.NewStyle().Padding(0, 1)
		if i == c.target {
			st = st.Background(lipgloss.Color("#303030"))
		}
		cards = append(cards, click(text(st.Render(chip(g)+" "+score)), calMsg{calTarget, i}))
	}
	return vjoin(cur, text(""), text(sDim.Render("All gestures (click one to test it):")), grid(w-6, 1, cards...))
}

func sourceWords(m Model, s gesture.Source) string {
	var names []string
	for _, st := range s.Strikers() {
		names = append(names, m.prof.Kit.Name(st))
	}
	if s.IsDuo() {
		return sDim.Render("both at once: " + strings.Join(names, " + "))
	}
	return sDim.Render("with " + names[0])
}

func (c calState) viewReport(m Model, w int) box {
	gs := m.prof.Kit.Gestures()
	reliable, tested := 0, 0
	type weak struct {
		g gesture.Gesture
		r float64
	}
	var weaks []weak
	for _, g := range gs {
		s := m.prof.Quality[g.String()]
		if s.Tries == 0 {
			continue
		}
		tested++
		if ratio(s) >= 2.0/3 {
			reliable++
		} else {
			weaks = append(weaks, weak{g, ratio(s)})
		}
	}
	sort.Slice(weaks, func(i, j int) bool { return weaks[i].r < weaks[j].r })
	head := fmt.Sprintf("%d of %d tested gestures are reliable → %d chords at your fingertips.", reliable, tested, reliable+reliable*reliable)
	mood := "Great start. Every reliable gesture multiplies your chords."
	if reliable < 6 {
		mood = "Every new kit feels odd at first. A few minutes of practice usually doubles this."
	}
	var tips []string
	for _, x := range weaks[:min(4, len(weaks))] {
		tips = append(tips, fmt.Sprintf("%s %s  %s", chip(x.g), pct(x.r), sDim.Render(x.g.Shape.HowTo())))
	}
	// per shape row, per source column
	srcs := m.prof.Kit.Sources()
	tbl := []string{sDim.Render(fmt.Sprintf("%-6s", "")) + " " + strings.Join(mapS(srcs, func(s gesture.Source) string {
		return lipgloss.NewStyle().Width(6).Foreground(srcColor(s)).Render(trunc(s.String(), 6))
	}), "")}
	for _, sh := range gesture.Shapes {
		row := fmt.Sprintf("%-6s ", sh)
		for _, s := range srcs {
			q := m.prof.Quality[gesture.Gesture{Src: s, Shape: sh}.String()]
			cell := sDim.Render("  -- ")
			if q.Tries > 0 {
				cell = pct(ratio(q)) + " "
			}
			row += lipgloss.NewStyle().Width(6).Render(cell)
		}
		tbl = append(tbl, row)
	}
	if len(srcs) > 12 {
		tbl = tbl[:0]
	}
	return vjoin(
		text(sTitle.Render("Your detection report")),
		text(sBold.Render(head)),
		text(sOK.Render(mood)),
		text(""),
		text(strings.Join(tbl, "\n")),
		text(""),
		text(separation(m, len(m.prof.Kit.Voices()))),
		text(""),
		text(sDim.Render("Weakest gestures:")),
		text(strings.Join(tips, "\n")),
		text(""),
		hjoin(1, button("Retest weak ones", calMsg{calRetestWeak, 0}), button("Test more", calMsg{calGo, int(calTest)}),
			hotButton("Start the Tutor →", tabMsg(tabTutor)), button("Explore Gestures", tabMsg(tabGestures))),
	)
}

func mapS[T any](xs []T, f func(T) string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}
