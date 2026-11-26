package app

import (
	"fmt"
	"math"
	"strings"

	"coinchord/internal/audio"
	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
	"coinchord/internal/kits"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

// The wire format: every message has "t". The browser renders it as is.

type gJSON struct {
	Name  string `json:"name"`
	Src   string `json:"src"`
	Shape string `json:"shape"`
	Hand  string `json:"hand"` // L, R or B (both)
	Glyph string `json:"glyph"`
	How   string `json:"how"`
}

func gestureJSON(g gesture.Gesture) gJSON {
	hand := "B"
	if !g.Src.IsDuo() {
		hand = g.Src.Strikers()[0].Hand().String()
	}
	return gJSON{g.String(), g.Src.String(), g.Shape.String(), hand, g.Shape.Glyph(), g.Shape.HowTo()}
}

type chordJSON struct {
	Name  string  `json:"name"`
	Steps []gJSON `json:"steps"`
}

func chordOf(c gesture.Chord) chordJSON {
	out := chordJSON{Name: c.String()}
	for _, g := range c.Steps() {
		out.Steps = append(out.Steps, gestureJSON(g))
	}
	return out
}

// bands are 0..1 levels of the 32 print bands on a -90..0 dB scale.
func bands(f dsp.Frame) []float64 {
	out := make([]float64, gesture.NB)
	for i := range out {
		lo, hi := gesture.BandRange(i)
		db := 20 * math.Log10(f.Band(lo, hi)/(dsp.BlockSize/4)+1e-9)
		out[i] = math.Round(math.Min(1, math.Max(0, (db+90)/90))*100) / 100
	}
	return out
}

func level(x float64) float64 { return math.Round((audio.DB(x)+90)/90*1000) / 1000 } // 0..1 on -90..0 dB

func frameJSON(a *App, f audio.Frame) []byte {
	return mustJSON(map[string]any{
		"t": "frame", "level": level(f.RMS), "peak": level(f.Peak),
		"noise": level(a.mon.Noise()), "trigger": level(a.prof.Tuning.Trigger), "bands": bands(f.Frame),
	})
}

type metricsJSON struct {
	DurMs    float64 `json:"durMs"`
	Peak     float64 `json:"peak"`
	Flat     float64 `json:"flat"`
	Bumps    int     `json:"bumps"`
	BendHz   float64 `json:"bendHz"`
	PeriodMs float64 `json:"periodMs"`
}

func strokeJSON(a *App, s audio.Stroke) []byte {
	m := s.Metrics
	return mustJSON(map[string]any{
		"t": "stroke", "gesture": gestureJSON(s.Gesture), "sims": s.Match.Sims, "best": s.Match.Best,
		"metrics": metricsJSON{m.DurMs, m.Peak, m.Flat, m.Peaks, m.DriftHz, m.PeriodMs},
	})
}

type voiceJSON struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Hand string `json:"hand"`
}

type knobJSON struct {
	Name  string  `json:"name"`
	Help  string  `json:"help"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Step  float64 `json:"step"`
	Value float64 `json:"value"`
}

type rowJSON struct {
	Chord     chordJSON `json:"chord"`
	Show      string    `json:"show"`
	Src       string    `json:"src"`
	Reachable bool      `json:"reachable"`
}

func (a *App) stateJSON() []byte {
	k := a.prof.Kit
	var voices []voiceJSON
	for _, v := range k.Voices() {
		voices = append(voices, voiceJSON{v.Striker.String(), v.Name, v.Striker.Hand().String()})
	}
	var presets []map[string]any
	for _, p := range kits.Presets {
		pk, err := p.Kit()
		if err != nil {
			continue
		}
		var objs []string
		for _, v := range p.Voices {
			objs = append(objs, v.Object)
		}
		n := len(pk.Gestures())
		presets = append(presets, map[string]any{
			"name": p.Name, "surface": p.Surface, "feel": p.Feel, "objects": objs,
			"contrast": p.Contrast, "glide": p.Glide, "speed": p.Speed, "gestures": n, "chords": n + n*n,
		})
	}
	var setups []map[string]any
	for _, s := range a.prof.Setups {
		setups = append(setups, map[string]any{"name": s.Name, "calibrated": s.Calibrated, "strikers": len(s.Kit.Voices())})
	}
	var knobs []knobJSON
	for _, kn := range gesture.Knobs {
		t := a.prof.Tuning
		knobs = append(knobs, knobJSON{kn.Name, kn.Help, kn.Min, kn.Max, kn.Step, *kn.Of(&t)})
	}
	var gestures []map[string]any
	for _, g := range k.Gestures() {
		q := a.prof.Quality[g.String()]
		gestures = append(gestures, map[string]any{"g": gestureJSON(g), "heard": a.heard[g], "hits": q.Hits, "tries": q.Tries})
	}
	var layers []string
	for _, l := range a.prof.Layers.Layers {
		layers = append(layers, l.Name)
	}
	var rows []rowJSON
	if l := a.viewed(); l != nil {
		for _, c := range l.Sorted(k) {
			rows = append(rows, rowJSON{chordOf(c), l.Bind[c].Show(), l.Bind[c].Src(), layer.Reachable(k, c)})
		}
	}
	var switches []rowJSON
	for _, c := range a.prof.Layers.Global.Sorted(k) {
		switches = append(switches, rowJSON{chordOf(c), a.prof.Layers.Global.Bind[c].Show(), a.prof.Layers.Global.Bind[c].Src(), true})
	}
	rec := map[string]any{"listening": a.rec.listening}
	if a.rec.chord != nil {
		rec["chord"] = chordOf(*a.rec.chord)
	}
	var last any
	if a.last != nil {
		last = gestureJSON(a.last.Gesture)
	}
	var chord any
	if a.chord != nil {
		chord = chordOf(a.chord.Chord)
	}
	return mustJSON(map[string]any{
		"t":       "state",
		"kit":     map[string]any{"name": a.prof.Preset, "calibrated": a.prof.Calibrated, "voices": voices},
		"presets": presets, "setups": setups, "knobs": knobs, "gestures": gestures,
		"output": a.out != nil, "mic": a.micJSON(), "notice": a.notice,
		"next": a.nextJSON(), "journey": a.journeyJSON(),
		"cal": a.calJSON(), "tutor": a.tutJSON(),
		"layers": layers, "view": a.view, "rows": rows, "switches": switches,
		"sandbox": map[string]any{"active": a.sand.Active, "text": a.sand.Text(), "host": a.sand.Host[max(0, len(a.sand.Host)-8):]},
		"rec":     rec, "last": last, "chord": chord, "typed": a.typed,
		"stories": a.opt.StoriesDir, "missing": a.missingWords(),
		"commands": commandNames(),
	})
}

func commandNames() []string {
	var out []string
	for _, c := range layer.Commands() {
		out = append(out, c.String())
	}
	return out
}

func (a *App) micJSON() map[string]any {
	state := map[audio.MicState]string{audio.MicWaiting: "waiting", audio.MicSilent: "silent", audio.MicClipping: "loud", audio.MicLive: "live"}[a.mon.State()]
	if a.micErr != "" {
		state = "off"
	}
	out := map[string]any{"state": state, "error": a.micErr, "volume": -1, "muted": false}
	if a.level != nil {
		out["volume"], out["muted"] = a.level.Volume, a.level.Muted
	}
	return out
}

func (a *App) calJSON() map[string]any {
	c := &a.cal
	vs := a.prof.Kit.Voices()
	taps := make([]int, len(vs))
	for i := range taps {
		if i < len(c.taps) {
			taps[i] = len(c.taps[i])
		}
	}
	out := map[string]any{
		"step": calStepNames[c.step], "voice": c.voice, "taps": taps, "tapsNeeded": tapsNeeded,
		"drags": len(c.drags), "dragsNeeded": dragsNeeded, "quiet": float64(c.quiet.Frames()) / quietFrames,
		"msg": c.msg, "good": c.good, "attempts": c.attempt, "triesEach": triesEach, "target": c.target,
	}
	// how alike the learned strikers sound: high = easy to confuse
	var alike []map[string]any
	for i := range vs {
		for j := i + 1; j < len(vs); j++ {
			alike = append(alike, map[string]any{"a": vs[i].Striker.String(), "b": vs[j].Striker.String(),
				"sim": math.Round(gesture.Similarity(vs[i].Print, vs[j].Print)*100) / 100})
		}
	}
	out["alike"] = alike
	reliable, tested := 0, 0
	for _, g := range a.prof.Kit.Gestures() {
		if s := a.prof.Quality[g.String()]; s.Tries > 0 {
			tested++
			if ratio(s) >= reliableRate {
				reliable++
			}
		}
	}
	out["reliable"], out["tested"], out["chords"] = reliable, tested, reliable+reliable*reliable
	return out
}

func (a *App) tutJSON() map[string]any {
	t := &a.tut
	var stories []map[string]any
	for _, s := range a.opt.Stories {
		ss := s.Sentences()
		best := make([]float64, len(ss)+1)
		for _, r := range a.prof.Records {
			if r.Story == s.Title && r.Lesson >= 0 && r.Lesson < len(best) {
				best[r.Lesson] = math.Max(best[r.Lesson], math.Round(r.WPM))
			}
		}
		stories = append(stories, map[string]any{"title": s.Title, "sentences": ss, "best": best})
	}
	var drills []map[string]any
	for _, d := range tutor.Drills {
		drills = append(drills, map[string]any{"title": d.Title, "intro": d.Intro})
	}
	var steps []map[string]any
	for _, s := range t.steps {
		steps = append(steps, map[string]any{"chord": chordOf(s.Chord), "show": s.Expand.Show(), "say": s.Say})
	}
	var progress []float64
	for _, r := range a.prof.Records[max(0, len(a.prof.Records)-30):] {
		progress = append(progress, math.Round(r.WPM))
	}
	var misses []string
	for k, v := range t.stats.Misses {
		misses = append(misses, fmt.Sprintf("%s ×%d", k, v))
	}
	out := map[string]any{
		"mode": tutModeNames[t.mode], "story": t.story, "lesson": t.lesson, "drill": t.drill,
		"stories": stories, "drills": drills, "steps": steps, "pos": t.pos,
		"typed": t.ty.Text(), "target": t.target, "msg": t.msg, "good": t.good,
		"stats": map[string]any{"wpm": math.Round(t.stats.WPM()), "cpm": math.Round(t.stats.CPM()), "accuracy": t.stats.Accuracy(), "wrong": t.stats.Wrong},
		"last":  t.last, "progress": progress, "misses": misses,
	}
	// the magic: how few gestures type the first sentence of the story
	if ss := a.opt.Stories[t.story].Sentences(); len(ss) > 0 {
		if plan, err := tutor.Plan(&a.prof.Layers, a.prof.Kit, "english", ss[0]); err == nil && len(plan) > 0 {
			out["magic"] = map[string]any{"chord": chordOf(plan[0].Chord), "show": plan[0].Expand.Show(), "count": len(plan), "sentence": strings.TrimSpace(ss[0])}
		}
	}
	return out
}
