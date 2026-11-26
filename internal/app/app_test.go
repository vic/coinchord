package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
	"coinchord/internal/inject"
	"coinchord/internal/kits"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

func newTest(t *testing.T, preset int) *App {
	t.Helper()
	p, err := config.ForPreset(kits.Presets[preset])
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	return New(p, Options{ProfilePath: filepath.Join(dir, "p.json"), StoriesDir: dir, Stories: tutor.Builtin})
}

func do(t *testing.T, a *App, cmds ...Cmd) {
	t.Helper()
	for _, c := range cmds {
		a.notice = ""
		a.onCmd(c)
		if a.notice != "" && strings.Contains(a.notice, "nknown") {
			t.Fatalf("%s: %s", c.Do, a.notice)
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
	a := newTest(t, 0)
	do(t, a, Cmd{Do: "page", Name: "calibrate"}, Cmd{Do: "cal.go", I: int(calQuiet)})
	for range quietFrames {
		a.onEvent(audio.Frame{Frame: dsp.Frame{RMS: 0.002, Peak: 0.01}})
	}
	if a.cal.step != calVoice || a.prof.Tuning.Trigger != 0.0076 {
		t.Fatalf("after quiet: step %d trigger %v", a.cal.step, a.prof.Tuning.Trigger)
	}
	k := a.prof.Kit
	for v := range k.Voices() {
		for range tapsNeeded {
			a.onEvent(stroke(k, v, gesture.Tap, 50))
		}
	}
	if a.cal.step != calDrag {
		t.Fatalf("after taps: step %d %q", a.cal.step, a.cal.msg)
	}
	for range dragsNeeded {
		a.onEvent(stroke(k, 0, gesture.Line, 300))
	}
	if !a.prof.Calibrated || a.cal.step != calTest || a.prof.Tuning.ScrapeFlat != 0.35 {
		t.Fatalf("after drags: %v %d %v", a.prof.Calibrated, a.cal.step, a.prof.Tuning.ScrapeFlat)
	}
	want := a.prof.Kit.Gestures()[a.cal.target]
	a.onEvent(audio.Stroke{Gesture: want})
	if s := a.prof.Quality[want.String()]; s.Hits != 1 || s.Tries != 1 {
		t.Fatalf("quality %+v", s)
	}
	// chords never act while calibrating
	a.onEvent(audio.Chord{Chord: gesture.Single(want)})
	if a.sand.Text() != "" {
		t.Fatal("calibrate must not type")
	}
	if saved, err := config.Load(a.opt.ProfilePath); err != nil || !saved.Calibrated {
		t.Fatalf("not saved: %v", err)
	}
}

func TestTutorLesson(t *testing.T) {
	a := newTest(t, 0)
	do(t, a, Cmd{Do: "page", Name: "tutor"}, Cmd{Do: "tut.lesson", I: 0})
	if a.tut.mode != tutPractice || len(a.tut.steps) == 0 {
		t.Fatalf("mode %d steps %d %q", a.tut.mode, len(a.tut.steps), a.tut.msg)
	}
	a.onEvent(audio.Chord{Chord: a.tut.steps[1].Chord})
	if a.tut.pos != 0 || a.tut.stats.Wrong != 1 || a.tut.ty.Text() != "" {
		t.Fatal("a wrong chord must not type")
	}
	for _, s := range a.tut.steps {
		a.now = a.now.Add(time.Second)
		a.onEvent(audio.Chord{Chord: s.Chord})
	}
	if a.tut.mode != tutDone || len(a.prof.Records) != 1 || a.prof.Records[0].WPM <= 0 {
		t.Fatalf("mode %d records %v", a.tut.mode, a.prof.Records)
	}
}

func TestRecordBindingAndSetups(t *testing.T) {
	a := newTest(t, 0)
	do(t, a, Cmd{Do: "page", Name: "settings"}, Cmd{Do: "layer.view", Name: "english"}, Cmd{Do: "bind.record"})
	c := gesture.Pair(a.prof.Kit.Gestures()[20], a.prof.Kit.Gestures()[19])
	a.onEvent(audio.Chord{Chord: c})
	if a.rec.chord == nil || *a.rec.chord != c || a.sand.Text() != "" {
		t.Fatal("recording must capture the chord, not type it")
	}
	do(t, a, Cmd{Do: "bind.save", Chord: c.String(), Text: "hello{enter}"})
	if l, _ := a.prof.Layers.Find("english"); l.Bind[c].Src() != "hello{enter}" {
		t.Fatal("binding not saved")
	}
	a.onCmd(Cmd{Do: "bind.save", Chord: c.String(), Text: "{nope}"})
	if a.notice == "" {
		t.Fatal("bad expansion must be refused")
	}
	do(t, a, Cmd{Do: "kit.add", I: 1}, Cmd{Do: "kit.add", I: 1}, Cmd{Do: "setup.save", Name: "plate four"}, Cmd{Do: "kit.pick", I: 7})
	if len(a.prof.Kit.Voices()) != 10 {
		t.Fatal("pick")
	}
	do(t, a, Cmd{Do: "setup.use", I: 0})
	if len(a.prof.Kit.Voices()) != 4 || a.prof.Preset != "plate four" {
		t.Fatalf("setup: %d %s", len(a.prof.Kit.Voices()), a.prof.Preset)
	}
	a.onCmd(Cmd{Do: "knob", I: 99, Value: 1})
	if a.notice == "" {
		t.Fatal("bad knob index must be refused")
	}
}

type fakeDevice struct{ log chan string }

func (f fakeDevice) Text(s string) error       { f.log <- "text:" + s; return nil }
func (f fakeDevice) Combo(c layer.Combo) error { f.log <- "key:" + c.String(); return nil }
func (f fakeDevice) Mouse(m layer.Mouse) error { f.log <- "mouse:" + m.String(); return nil }
func (f fakeDevice) Close() error              { return nil }

func TestOutputDrill(t *testing.T) {
	a := newTest(t, 0)
	dev := fakeDevice{make(chan string, 256)}
	a.outReady(openOutput(func() (inject.Device, error) { return dev, nil }))
	if a.out == nil {
		t.Fatalf("output: %s", a.notice)
	}
	do(t, a, Cmd{Do: "page", Name: "tutor"}, Cmd{Do: "tut.drill", I: 0})
	a.onEvent(audio.Chord{Chord: a.tut.steps[1].Chord}) // wrong: nothing reaches the host
	for _, s := range a.tut.steps {
		a.onEvent(audio.Chord{Chord: s.Chord})
	}
	if a.tut.mode != tutDone {
		t.Fatalf("drill at %d/%d", a.tut.pos, len(a.tut.steps))
	}
	do(t, a, Cmd{Do: "output", Value: 0})
	var got []string
	for len(got) < 11 {
		select {
		case g := <-dev.log:
			got = append(got, g)
		case <-time.After(time.Second):
			t.Fatalf("host got %v", got)
		}
	}
	if got[0] != "key:alt+tab" || got[1] != "key:alt+shift+tab" || got[10] != "key:esc" {
		t.Fatalf("host got %v", got)
	}
}

// The full loop: clients get state first, frames and strokes stream, and
// commands come back through the same goroutine.
func TestRunLoop(t *testing.T) {
	a := newTest(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	ch, leave := a.Join()
	defer leave()
	kinds := func(n int) map[string]map[string]any {
		seen := map[string]map[string]any{}
		for range n {
			select {
			case b := <-ch:
				var m map[string]any
				if err := json.Unmarshal(b, &m); err != nil {
					t.Fatal(err)
				}
				seen[m["t"].(string)] = m
			case <-time.After(time.Second):
				t.Fatalf("waited; got %v", seen)
			}
		}
		return seen
	}
	if s := kinds(1)["state"]; s == nil || s["next"] == nil {
		t.Fatal("first message must be the state")
	}
	a.Emit(audio.Frame{Frame: dsp.Frame{RMS: 0.01, Mag: make([]float32, dsp.BlockSize/2)}})
	a.Emit(stroke(a.prof.Kit, 0, gesture.Tap, 50))
	a.Do(Cmd{Do: "page", Name: "layers"})
	seen := map[string]bool{}
	for !(seen["frame"] && seen["stroke"] && seen["state+stroke"]) {
		for k, m := range kinds(1) {
			seen[k] = true
			if last, ok := m["last"].(map[string]any); k == "state" && ok && last["name"] == "L_TAP" && m["view"] != nil {
				seen["state+stroke"] = true
			}
		}
	}
}
