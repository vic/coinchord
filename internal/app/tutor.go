package app

import (
	"fmt"
	"strings"
	"time"

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

var tutModeNames = []string{"pick", "practice", "demo", "done"}

const demoStep = 600 * time.Millisecond

type tut struct {
	mode    tutMode
	story   int
	lesson  int // sentence index; len(sentences) = the whole story
	drill   int // index into tutor.Drills, -1 = a story lesson
	steps   []tutor.Step
	pos     int
	ty      layer.Typist
	target  string
	stats   tutor.Stats
	started bool
	msg     string
	good    bool
	demoAt  time.Time
	last    tutor.Record
}

func (t *tut) running() bool { return t.mode == tutPractice }

func (t *tut) sentences(a *App) []string { return a.opt.Stories[t.story].Sentences() }

func (t *tut) reset(mode tutMode, now time.Time) {
	t.mode, t.pos, t.msg, t.started = mode, 0, "", false
	t.ty = layer.Typist{Active: "english"}
	t.steps, t.target = nil, ""
	t.demoAt, t.stats = now, tutor.NewStats(now)
}

func (t *tut) start(a *App, lesson int, mode tutMode) {
	ss := t.sentences(a)
	if lesson < 0 || lesson > len(ss) {
		return
	}
	t.reset(mode, a.now)
	t.lesson, t.drill = lesson, -1
	part := ss
	if lesson < len(ss) {
		part = ss[lesson : lesson+1]
	}
	t.target = strings.Join(part, "")
	for _, s := range part {
		steps, err := tutor.Plan(&a.prof.Layers, a.prof.Kit, "english", s)
		if err != nil {
			t.mode, t.msg = tutPick, "This text needs a chord you don't have yet. Add one in Settings → Chords."
			return
		}
		t.steps = append(t.steps, steps...)
	}
}

func (t *tut) startDrill(a *App, i int) {
	if i < 0 || i >= len(tutor.Drills) {
		return
	}
	t.reset(tutPractice, a.now)
	t.drill = i
	steps, err := tutor.PlanDrill(&a.prof.Layers, a.prof.Kit, "english", tutor.Drills[i])
	if err != nil {
		t.mode, t.msg = tutPick, "This drill needs a chord you don't have: "+err.Error()
		return
	}
	t.steps = steps
}

func (t *tut) tick(a *App) {
	if t.mode != tutDemo || t.pos >= len(t.steps) || a.now.Sub(t.demoAt) < demoStep {
		return
	}
	t.ty.Apply(t.steps[t.pos].Expand)
	t.pos++
	t.demoAt = a.now
	a.dirty = true
}

func (t *tut) onChord(a *App, c gesture.Chord) {
	if t.pos >= len(t.steps) {
		return
	}
	if !t.started {
		t.started, t.stats = true, tutor.NewStats(a.now)
	}
	want := t.steps[t.pos]
	if c != want.Chord {
		t.stats.Miss(want.Chord, a.now)
		t.good = false
		t.msg = fmt.Sprintf("I heard %s. No problem — try %s again.", c, want.Chord)
		if gs, ws := c.Steps(), want.Chord.Steps(); len(gs) == 1 && len(ws) == 1 {
			t.msg += " " + hint(ws[0], gs[0])
		}
		return
	}
	before := len([]rune(t.ty.Text()))
	t.ty.Apply(want.Expand)
	a.emit(want.Expand)
	t.stats.Hit(max(0, len([]rune(t.ty.Text()))-before), a.now)
	t.pos++
	t.good, t.msg = true, cheers[t.pos%len(cheers)]
	if t.pos == len(t.steps) {
		t.finish(a)
	}
}

var cheers = []string{"Yes!", "Perfect.", "Nice!", "Great one.", "Smooth."}

func (t *tut) finish(a *App) {
	t.mode = tutDone
	title := a.opt.Stories[t.story].Title
	if t.drill >= 0 {
		title = "drill: " + tutor.Drills[t.drill].Title
	}
	t.last = t.stats.Record(title, t.lesson)
	a.prof.Records = append(a.prof.Records, t.last)
	a.save()
}
