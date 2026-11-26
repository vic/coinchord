package tui

import (
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
	"coinchord/internal/kits"
	"coinchord/internal/tutor"
)

func newTest(t *testing.T, preset int) Model {
	t.Helper()
	p, err := config.ForPreset(kits.Presets[preset])
	if err != nil {
		t.Fatal(err)
	}
	var setup atomic.Pointer[audio.Setup]
	dir := t.TempDir()
	m := New(p, Options{ProfilePath: filepath.Join(dir, "p.json"), StoriesDir: dir, Stories: tutor.Builtin, Setup: &setup})
	return send(m, tea.WindowSizeMsg{Width: 160, Height: 50})
}

func send(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		mm, _ := m.Update(msg)
		m = mm.(Model)
	}
	return m
}

// clickOn finds the zone that sends want and clicks its top-left cell.
func clickOn(t *testing.T, m Model, want tea.Msg) Model {
	t.Helper()
	all := m.screen()
	for _, z := range all.zones {
		if z.msg == want {
			got, ok := all.hit(z.x, z.y)
			if !ok {
				t.Fatalf("no hit at %d,%d", z.x, z.y)
			}
			return send(m, got)
		}
	}
	t.Fatalf("no zone sends %#v", want)
	return m
}

func TestEveryTabRendersAndIsClickable(t *testing.T) {
	for _, preset := range []int{0, 7} {
		m := newTest(t, preset)
		for i := range tabCount {
			m = clickOn(t, m, tabMsg(i))
			if m.tab != i {
				t.Fatalf("click tab %d: at %d", i, m.tab)
			}
			if s := m.screen().s; !strings.Contains(s, tabNames[i]) || len(s) < 200 {
				t.Fatalf("tab %s did not render", tabNames[i])
			}
		}
	}
}

func stroke(k gesture.Kit, i int, shape gesture.Shape, dur float64) audio.Stroke {
	v := k.Voices()[i]
	m := gesture.Metrics{DurMs: dur, Peak: 0.1, Flat: 0.1, Print: v.Print}
	if shape != gesture.Tap {
		m.Flat = 0.6
	}
	return audio.Stroke{Metrics: m, Gesture: gesture.Gesture{Src: gesture.Solo(v.Striker), Shape: shape}}
}

func TestCalibrateFlow(t *testing.T) {
	m := newTest(t, 0)
	m = clickOn(t, m, tabMsg(tabCalibrate))
	m = clickOn(t, m, calMsg{calGo, int(calQuiet)})
	for range quietFrames {
		m = send(m, audio.Frame{Frame: dsp.Frame{RMS: 0.002}})
	}
	if m.cal.step != calVoice || m.prof.Tuning.Trigger != 0.0076 {
		t.Fatalf("after quiet: step %d trigger %v", m.cal.step, m.prof.Tuning.Trigger)
	}
	k := m.prof.Kit
	for v := range k.Voices() {
		for range tapsNeeded {
			m = send(m, stroke(k, v, gesture.Tap, 50))
		}
	}
	if m.cal.step != calScrape {
		t.Fatalf("after taps: step %d msg %q", m.cal.step, m.cal.msg)
	}
	for range dragsNeeded {
		m = send(m, stroke(k, 0, gesture.Line, 300))
	}
	if !m.prof.Calibrated || m.cal.step != calTest || m.prof.Tuning.ScrapeFlat != 0.35 {
		t.Fatalf("after drags: calibrated %v step %d flat %v", m.prof.Calibrated, m.cal.step, m.prof.Tuning.ScrapeFlat)
	}
	// the test step scores what it hears
	want := m.prof.Kit.Gestures()[m.cal.target]
	m = send(m, audio.Stroke{Gesture: want})
	if s := m.prof.Quality[want.String()]; s.Hits != 1 || s.Tries != 1 {
		t.Fatalf("quality %+v", s)
	}
	saved, err := config.Load(m.opt.ProfilePath)
	if err != nil || !saved.Calibrated {
		t.Fatalf("profile not saved: %v", err)
	}
}

func TestTutorLesson(t *testing.T) {
	m := newTest(t, 0)
	m = clickOn(t, m, tabMsg(tabTutor))
	m = clickOn(t, m, tutMsg{tutLesson, 0})
	if m.tut.mode != tutPractice || len(m.tut.steps) == 0 {
		t.Fatalf("mode %d steps %d msg %q", m.tut.mode, len(m.tut.steps), m.tut.msg)
	}
	wrong := m.tut.steps[1].Chord
	m = send(m, audio.Chord{Chord: wrong})
	if m.tut.pos != 0 || m.tut.stats.Wrong != 1 || m.tut.ty.Text() != "" {
		t.Fatal("a wrong chord must not type")
	}
	for i, s := range m.tut.steps {
		m.now = m.now.Add(time.Second)
		m = send(m, audio.Chord{Chord: s.Chord})
		if m.tut.pos != i+1 && m.tut.mode != tutDone {
			t.Fatalf("step %d not accepted", i)
		}
	}
	if m.tut.mode != tutDone || len(m.prof.Records) != 1 {
		t.Fatalf("mode %d records %d", m.tut.mode, len(m.prof.Records))
	}
	first := tutor.Builtin[0].Sentences()[0]
	if !strings.EqualFold(strings.TrimSpace(m.tut.ty.Text()), strings.TrimSpace(first)) {
		t.Fatalf("typed %q", m.tut.ty.Text())
	}
	if r := m.prof.Records[0]; r.WPM <= 0 || r.Accuracy >= 1 {
		t.Fatalf("record %+v", r)
	}
}

func TestRecordBinding(t *testing.T) {
	m := newTest(t, 0)
	m = clickOn(t, m, tabMsg(tabSettings))
	m = clickOn(t, m, setMsg{act: setPageAct, i: int(pageBinds)})
	m = clickOn(t, m, setMsg{act: setRecord})
	c := gesture.Pair(m.prof.Kit.Gestures()[20], m.prof.Kit.Gestures()[19])
	m = send(m, audio.Chord{Chord: c})
	if m.set.mode != modeExpansion {
		t.Fatalf("mode %d", m.set.mode)
	}
	m.set.input.SetValue("hello world{enter}")
	m = clickOn(t, m, setMsg{act: setSave})
	l, _ := m.prof.Layers.Find("english")
	if e, ok := l.Bind[c]; !ok || e.Src() != "hello world{enter}" {
		t.Fatalf("binding not saved: %v", m.set.err)
	}
	m = clickOn(t, m, setMsg{act: setRecord})
	m = send(m, audio.Chord{Chord: c})
	m.set.input.SetValue("{nope}")
	m = clickOn(t, m, setMsg{act: setSave})
	if m.set.err == "" || m.set.mode != modeExpansion {
		t.Fatal("invalid expansion must be refused and keep the editor open")
	}
}
