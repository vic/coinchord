package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"coinchord/internal/gesture"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

type tutMode uint8

const (
	tutPick tutMode = iota
	tutPractice
	tutDemo
	tutDone
)

type tutAct uint8

const (
	tutStory tutAct = iota
	tutLesson
	tutWatch
	tutAgain
	tutNext
	tutBack
)

type tutMsg struct {
	act tutAct
	i   int
}

const demoStep = 550 * time.Millisecond

type tutState struct {
	mode    tutMode
	story   int
	lesson  int // sentence index; == len(sentences) means the whole story
	steps   []tutor.Step
	pos     int
	ty      layer.Typist
	target  string
	stats   tutor.Stats
	started bool
	msg     string
	good    bool
	demoAt  time.Time
	top     int
	last    tutor.Record
}

func (t *tutState) on(m *Model, msg tutMsg) {
	switch msg.act {
	case tutStory:
		t.story, t.lesson, t.mode = msg.i, 0, tutPick
	case tutLesson:
		t.start(m, msg.i, tutPractice)
	case tutWatch:
		t.start(m, msg.i, tutDemo)
	case tutAgain:
		t.start(m, t.lesson, tutPractice)
	case tutNext:
		t.start(m, min(t.lesson+1, len(t.sentences(m))), tutPractice)
	case tutBack:
		t.mode = tutPick
	}
}

func (t *tutState) sentences(m *Model) []string {
	return m.opt.Stories[t.story].Sentences()
}

func (t *tutState) start(m *Model, lesson int, mode tutMode) {
	ss := t.sentences(m)
	t.lesson, t.mode, t.pos, t.msg, t.started = lesson, mode, 0, "", false
	t.ty = layer.Typist{Active: "english"}
	t.steps = nil
	part := ss
	if lesson < len(ss) {
		part = ss[lesson : lesson+1]
	}
	t.target = strings.Join(part, "")
	for _, s := range part {
		steps, err := tutor.Plan(&m.prof.Layers, m.prof.Kit, "english", s)
		if err != nil {
			t.mode, t.msg = tutPick, "This text needs a binding you do not have yet: "+err.Error()+". Add one in Settings → Bindings."
			return
		}
		t.steps = append(t.steps, steps...)
	}
	t.demoAt = m.now
	t.stats = tutor.NewStats(m.now)
}

func (t *tutState) onTick(m *Model) {
	if m.tab != tabTutor || t.mode != tutDemo || t.pos >= len(t.steps) || m.now.Sub(t.demoAt) < demoStep {
		return
	}
	t.ty.Apply(t.steps[t.pos].Expand)
	t.pos++
	t.demoAt = m.now
}

func (t *tutState) onChord(m *Model, c gesture.Chord) {
	if t.mode != tutPractice || t.pos >= len(t.steps) {
		return
	}
	if !t.started {
		t.started, t.stats = true, tutor.NewStats(m.now)
	}
	want := t.steps[t.pos]
	if c != want.Chord {
		t.stats.Miss(want.Chord, m.now)
		t.good = false
		t.msg = fmt.Sprintf("I heard %s (%s). That's fine, try %s again.", chordChip(c), lookupTyped(&m.prof.Layers, t.ty.Active, c), chordChip(want.Chord))
		if gs, ws := c.Steps(), want.Chord.Steps(); len(gs) == len(ws) && len(gs) == 1 {
			t.msg += " " + hint(ws[0], gs[0])
		}
		return
	}
	before := len([]rune(t.ty.Text()))
	t.ty.Apply(want.Expand)
	t.stats.Hit(max(0, len([]rune(t.ty.Text()))-before), m.now)
	t.pos++
	t.good, t.msg = true, praise[t.pos%len(praise)]
	t.msg = fmt.Sprintf(t.msg, chordChip(c))
	if t.pos == len(t.steps) {
		t.finish(m)
	}
}

func (t *tutState) finish(m *Model) {
	t.mode = tutDone
	t.last = t.stats.Record(m.opt.Stories[t.story].Title, t.lesson)
	m.prof.Records = append(m.prof.Records, t.last)
	m.save()
}

func (t tutState) view(m Model, w, h int) box {
	stories := m.opt.Stories
	var list []box
	list = append(list, text(sBold.Render("Stories")))
	for i, s := range stories {
		st := sDim
		if i == t.story {
			st = sOK.Bold(true)
		}
		list = append(list, click(text(st.Render("▸ "+trunc(s.Title, 24))), tutMsg{tutStory, i}))
	}
	list = append(list, text(""), text(sBold.Render("Lessons")))
	ss := stories[t.story].Sentences()
	for i, s := range ss {
		st := sDim
		if i == t.lesson && t.mode != tutPick {
			st = sOK.Bold(true)
		}
		best := bestWPM(m, stories[t.story].Title, i)
		list = append(list, click(text(st.Render(fmt.Sprintf("%2d %s", i+1, trunc(s, 20)))+best), tutMsg{tutLesson, i}))
	}
	list = append(list, click(text(sWarn.Render("★ Whole story")+bestWPM(m, stories[t.story].Title, len(ss))), tutMsg{tutLesson, len(ss)}))
	left := wrap(sPanel.Width(34), vjoin(list...))
	right := w - 38
	var main box
	switch t.mode {
	case tutPick:
		main = t.viewIntro(m, right)
	case tutDone:
		main = t.viewDone(m, right)
	default:
		main = t.viewRun(m, right)
	}
	return hjoin(1, left, wrap(sPanel.Width(right), main))
}

func bestWPM(m Model, story string, lesson int) string {
	best := 0.0
	for _, r := range m.prof.Records {
		if r.Story == story && r.Lesson == lesson && r.WPM > best {
			best = r.WPM
		}
	}
	if best == 0 {
		return ""
	}
	return sDim.Render(fmt.Sprintf(" %.0f", best))
}

func (t tutState) viewIntro(m Model, w int) box {
	set := &m.prof.Layers
	first := m.opt.Stories[t.story].Sentences()[0]
	steps, err := tutor.Plan(set, m.prof.Kit, "english", first)
	magic := sDim.Render("(no plan: " + fmt.Sprint(err) + ")")
	if err == nil && len(steps) > 0 {
		magic = fmt.Sprintf("%s  →  %s\n\n%d small gestures type the whole sentence (%d letters):\n%s",
			chordChip(steps[0].Chord), sBold.Render("\""+steps[0].Expand.Show()+"\""),
			len(steps), len(first), sOK.Render(first))
	}
	var msg box
	if t.msg != "" {
		msg = text(sWarn.Render(t.msg))
	}
	return vjoin(
		text(sTitle.Render("Small gestures, whole phrases")),
		text(""),
		text(magic),
		text(""),
		hjoin(1, hotButton("▶ Watch the magic", tutMsg{tutWatch, 0}), button("Try it myself", tutMsg{tutLesson, 0})),
		text(""),
		text(lipgloss.NewStyle().Width(w-4).Render(sDim.Render(
			"Each lesson is one sentence. The tutor shows the next chord, how to make it, and what it types. "+
				"Wrong chords never type anything here, so mistakes cost nothing. Say each word out loud as you make its shape. "+
				"Add your own texts: put .txt files in "+m.opt.StoriesDir))),
		msg,
		text(""),
		text(progress(m)),
	)
}

func (t tutState) viewRun(m Model, w int) box {
	tr := []rune(t.target)
	n := min(len([]rune(t.ty.Text())), len(tr))
	sentence := sOK.Bold(true).Render(string(tr[:n])) + sDim.Render(string(tr[n:]))
	parts := []box{
		text(sTitle.Render(map[tutMode]string{tutDemo: "Watch", tutPractice: "Your turn"}[t.mode]) +
			sDim.Render(fmt.Sprintf("   chord %d of %d · layer %s", min(t.pos+1, len(t.steps)), len(t.steps), t.ty.Active))),
		text(""),
		text(lipgloss.NewStyle().Width(w - 4).Render(sentence)),
		text(""),
	}
	if t.pos < len(t.steps) {
		s := t.steps[t.pos]
		g := s.Chord.Steps()[0]
		what := sBold.Render("\"" + s.Expand.Show() + "\"")
		parts = append(parts, hjoin(3, text(art(g.Shape, srcColor(g.Src))), vjoin(
			text("next  "+chordChip(s.Chord)),
			text("types "+what),
			text(lipgloss.NewStyle().Width(w-20).Render(sDim.Render(chordHow(m, s.Chord)))),
		)))
		var soon []string
		for _, u := range t.steps[t.pos+1 : min(len(t.steps), t.pos+5)] {
			soon = append(soon, chordChip(u.Chord)+sDim.Render(" "+u.Expand.Show()))
		}
		parts = append(parts, text(sDim.Render("then  ")+strings.Join(soon, sDim.Render("  ·  "))))
	} else if t.mode == tutDemo {
		parts = append(parts, text(sOK.Render(fmt.Sprintf("That's it: %d gestures typed %d characters. Your turn!", len(t.steps), len(t.target)))),
			hotButton("Try it myself", tutMsg{tutLesson, t.lesson}))
	}
	if t.msg != "" {
		st := sWarn
		if t.good {
			st = sOK
		}
		parts = append(parts, text(""), text(st.Render(t.msg)))
	}
	if t.mode == tutPractice {
		parts = append(parts, text(""), text(liveStats(t.stats)))
	}
	parts = append(parts, text(""), hjoin(1, button("Restart", tutMsg{tutAgain, 0}), button("Lessons", tutMsg{tutBack, 0})))
	return vjoin(parts...)
}

func chordHow(m Model, c gesture.Chord) string {
	var how []string
	for i, g := range c.Steps() {
		h := sourceWords(m, g.Src) + ": " + g.Shape.HowTo()
		if i == 1 {
			h = "then quickly " + h
		}
		how = append(how, h)
	}
	return strings.Join(how, " ")
}

func liveStats(s tutor.Stats) string {
	return fmt.Sprintf("%s %.0f   %s %.0f   %s %s   %s %d",
		sDim.Render("WPM"), s.WPM(), sDim.Render("chords/min"), s.CPM(), sDim.Render("accuracy"), pct(s.Accuracy()), sDim.Render("misses"), s.Wrong)
}

func (t tutState) viewDone(m Model, w int) box {
	r := t.last
	var misses []string
	type kv struct {
		k string
		v int
	}
	var ms []kv
	for k, v := range t.stats.Misses {
		ms = append(ms, kv{k, v})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].v > ms[j].v })
	for _, x := range ms[:min(3, len(ms))] {
		misses = append(misses, fmt.Sprintf("%s ×%d", x.k, x.v))
	}
	cheer := "Done! Every run builds the motion into your hands."
	if r.Accuracy >= 0.9 {
		cheer = "Excellent run! Your hands know these words now."
	}
	parts := []box{
		text(sTitle.Render("Finished: ") + sOK.Render(t.target)),
		text(""),
		text(sOK.Bold(true).Render(cheer)),
		text(""),
		text(fmt.Sprintf("%s %s   %s %s   %s %s   %s %s",
			sDim.Render("WPM"), sBold.Render(fmt.Sprintf("%.0f", r.WPM)), sDim.Render("chords/min"), sBold.Render(fmt.Sprintf("%.0f", r.CPM)),
			sDim.Render("accuracy"), pct(r.Accuracy), sDim.Render("time"), sBold.Render(fmt.Sprintf("%.1fs", r.Seconds)))),
	}
	if len(misses) > 0 {
		parts = append(parts, text(sDim.Render("practice these: ")+strings.Join(misses, ", ")))
	}
	parts = append(parts, text(""), text(progress(m)), text(""),
		hjoin(1, button("Again", tutMsg{tutAgain, 0}), hotButton("Next →", tutMsg{tutNext, 0}), button("Lessons", tutMsg{tutBack, 0})))
	return vjoin(parts...)
}

// progress is a sparkline of the last runs' WPM.
func progress(m Model) string {
	rs := m.prof.Records
	if len(rs) == 0 {
		return sDim.Render("Your progress chart appears after your first run.")
	}
	rs = rs[max(0, len(rs)-30):]
	top := 1.0
	for _, r := range rs {
		top = max(top, r.WPM)
	}
	var sb strings.Builder
	for _, r := range rs {
		sb.WriteRune(blocks[1+int(r.WPM/top*7)])
	}
	last := rs[len(rs)-1]
	return fmt.Sprintf("%s %s  %s %.0f WPM · best %.0f", sDim.Render("progress"), sOK.Render(sb.String()), sDim.Render("last"), last.WPM, top)
}
