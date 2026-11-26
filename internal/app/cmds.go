package app

import (
	"fmt"
	"slices"
	"strings"

	"coinchord/internal/audio"
	"coinchord/internal/gesture"
	"coinchord/internal/kits"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

// page is what the browser shows; it decides where chords go.
type page string

const (
	pageKits      page = "kits"
	pageCalibrate page = "calibrate"
	pageGestures  page = "gestures"
	pageTutor     page = "tutor"
	pageLayers    page = "layers"
	pageSettings  page = "settings"
)

var pages = []page{pageKits, pageCalibrate, pageGestures, pageTutor, pageLayers, pageSettings}

// Cmd is one request from the browser. Do names the handler; the other
// fields are its arguments and are checked by the handler.
type Cmd struct {
	Do    string  `json:"do"`
	I     int     `json:"i"`
	Value float64 `json:"value"`
	Name  string  `json:"name"`
	Text  string  `json:"text"`
	Chord string  `json:"chord"`
}

func (a *App) onCmd(c Cmd) {
	h, ok := handlers[c.Do]
	if !ok {
		a.notice = "Unknown command " + c.Do
	} else if err := h(a, c); err != nil {
		a.notice = err.Error()
	}
	a.dirty = true
}

func index[T any](xs []T, i int, what string) error {
	if i < 0 || i >= len(xs) {
		return fmt.Errorf("no %s %d", what, i)
	}
	return nil
}

var handlers = map[string]func(*App, Cmd) error{
	"page": func(a *App, c Cmd) error {
		p := page(c.Name)
		if !slices.Contains(pages, p) {
			return fmt.Errorf("unknown page %q", c.Name)
		}
		a.page = p
		return nil
	},
	"notice.clear": func(a *App, c Cmd) error { a.notice = ""; return nil },
	"output":       func(a *App, c Cmd) error { a.setOutput(c.Value != 0); return nil },
	"mic": func(a *App, c Cmd) error {
		act := audio.MicAction(c.I)
		if act > audio.Louder {
			return fmt.Errorf("unknown mic action %d", c.I)
		}
		go func() {
			if err := audio.Control(act); err != nil {
				a.Emit(noticeMsg(err.Error()))
			}
			a.readLevel()
		}()
		return nil
	},

	// kits and setups
	"kit.pick": func(a *App, c Cmd) error {
		if err := index(kits.Presets, c.I, "kit"); err != nil {
			return err
		}
		p, err := a.prof.WithPreset(kits.Presets[c.I])
		if err != nil {
			return err
		}
		p.Calibrated = false
		a.prof, a.cal = p, cal{}
		a.save()
		return nil
	},
	"kit.add": func(a *App, c Cmd) error {
		h := gesture.Hand(c.I)
		if h > gesture.Right {
			return fmt.Errorf("no hand %d", c.I)
		}
		return a.editKit(func(k gesture.Kit) (gesture.Kit, error) {
			return k.Add(h, "New "+strings.ToLower(h.Word())+" object", 8000)
		})
	},
	"kit.remove": func(a *App, c Cmd) error {
		return a.editKit(func(k gesture.Kit) (gesture.Kit, error) { return k.Remove(c.I) })
	},
	"kit.rename": func(a *App, c Cmd) error {
		return a.editKit(func(k gesture.Kit) (gesture.Kit, error) { return k.Rename(c.I, strings.TrimSpace(c.Name)) })
	},
	"setup.save": func(a *App, c Cmd) error {
		name := strings.TrimSpace(c.Name)
		p, err := a.prof.SaveSetup(name)
		if err != nil {
			return err
		}
		p.Preset, a.prof = name, p
		a.save()
		return nil
	},
	"setup.use": func(a *App, c Cmd) error {
		p, err := a.prof.UseSetup(c.I)
		if err != nil {
			return err
		}
		a.prof, a.cal = p, cal{}
		a.save()
		return nil
	},
	"setup.drop": func(a *App, c Cmd) error {
		a.prof = a.prof.DropSetup(c.I)
		a.save()
		return nil
	},

	// calibrate
	"cal.go": func(a *App, c Cmd) error {
		if c.I < 0 || c.I >= int(calSteps) {
			return fmt.Errorf("no calibration step %d", c.I)
		}
		a.cal.enter(a, calStep(c.I))
		return nil
	},
	"cal.skip": func(a *App, c Cmd) error { a.cal.next(a); return nil },
	"cal.target": func(a *App, c Cmd) error {
		if err := index(a.prof.Kit.Gestures(), c.I, "gesture"); err != nil {
			return err
		}
		a.cal.step, a.cal.target, a.cal.attempt, a.cal.msg = calTest, c.I, nil, ""
		return nil
	},
	"cal.retest": func(a *App, c Cmd) error { a.cal.retestWeak(a); return nil },

	// detection knobs
	"knob": func(a *App, c Cmd) error {
		if err := index(gesture.Knobs, c.I, "knob"); err != nil {
			return err
		}
		*gesture.Knobs[c.I].Of(&a.prof.Tuning) = c.Value
		a.prof.Tuning = a.prof.Tuning.Clamp()
		a.save()
		return nil
	},
	"knobs.reset": func(a *App, c Cmd) error {
		d, t := gesture.DefaultTuning(), a.prof.Tuning
		d.Trigger, d.TapPeak, d.ScrapeFlat = t.Trigger, t.TapPeak, t.ScrapeFlat // keep the calibrated values
		a.prof.Tuning = d
		a.save()
		return nil
	},
	"trigger.auto": func(a *App, c Cmd) error {
		a.prof.Tuning.Trigger = audio.TriggerFor(a.mon)
		a.prof.Tuning = a.prof.Tuning.Clamp()
		a.save()
		return nil
	},

	// tutor
	"tut.story": func(a *App, c Cmd) error {
		if err := index(a.opt.Stories, c.I, "story"); err != nil {
			return err
		}
		a.tut.story, a.tut.lesson, a.tut.mode, a.tut.drill = c.I, 0, tutPick, -1
		return nil
	},
	"tut.lesson": func(a *App, c Cmd) error { a.tut.start(a, c.I, tutPractice); return nil },
	"tut.watch":  func(a *App, c Cmd) error { a.tut.start(a, c.I, tutDemo); return nil },
	"tut.drill":  func(a *App, c Cmd) error { a.tut.startDrill(a, c.I); return nil },
	"tut.again": func(a *App, c Cmd) error {
		if a.tut.drill >= 0 {
			a.tut.startDrill(a, a.tut.drill)
		} else {
			a.tut.start(a, a.tut.lesson, tutPractice)
		}
		return nil
	},
	"tut.next": func(a *App, c Cmd) error {
		if a.tut.drill >= 0 {
			a.tut.startDrill(a, (a.tut.drill+1)%len(tutor.Drills))
		} else {
			a.tut.start(a, min(a.tut.lesson+1, len(a.tut.sentences(a))), tutPractice)
		}
		return nil
	},
	"tut.back": func(a *App, c Cmd) error { a.tut.mode = tutPick; return nil },

	// layers and the sandbox
	"layer.view": func(a *App, c Cmd) error {
		if c.Name != "global" {
			if _, ok := a.prof.Layers.Find(c.Name); !ok {
				return fmt.Errorf("no layer %q", c.Name)
			}
		}
		a.view = c.Name
		return nil
	},
	"layer.use": func(a *App, c Cmd) error {
		if _, ok := a.prof.Layers.Find(c.Name); !ok {
			return fmt.Errorf("no layer %q", c.Name)
		}
		a.sand.Active = c.Name
		return nil
	},
	"layer.play": func(a *App, c Cmd) error {
		ch, err := gesture.ParseChord(c.Chord)
		if err != nil {
			return err
		}
		a.apply(ch)
		return nil
	},
	"sandbox.clear": func(a *App, c Cmd) error {
		active := a.sand.Active
		a.sand.Reset()
		a.sand.Active = active
		return nil
	},
	"layer.new": func(a *App, c Cmd) error {
		name := strings.TrimSpace(c.Name)
		if _, dup := a.prof.Layers.Find(name); dup || name == "" || name == "global" || strings.ContainsAny(name, " {}:") {
			return fmt.Errorf("a layer name must be new, not empty, and without spaces, { } or :")
		}
		ch, ok := a.prof.Layers.Unused(a.prof.Kit)
		if !ok {
			return fmt.Errorf("no free chord left for a layer switch")
		}
		sw, err := layer.Parse("{layer:" + name + "}")
		if err != nil {
			return err
		}
		a.prof.Layers.Layers = append(a.prof.Layers.Layers, layer.Layer{Name: name, Bind: map[gesture.Chord]layer.Expansion{}})
		a.prof.Layers.Global.Bind[ch] = sw
		a.view = name
		a.notice = fmt.Sprintf("Layer %s is ready. Switch to it with %s.", name, ch)
		a.save()
		return nil
	},
	"layers.reset": func(a *App, c Cmd) error {
		set, err := layer.Defaults(a.prof.Kit, tutor.Words(a.opt.Stories))
		if err != nil {
			return err
		}
		a.prof.Layers = set
		a.save()
		return nil
	},

	// recording and editing chords
	"bind.record": func(a *App, c Cmd) error { a.rec = recording{listening: true}; return nil },
	"bind.cancel": func(a *App, c Cmd) error { a.rec = recording{}; return nil },
	"bind.save": func(a *App, c Cmd) error {
		ch, err := gesture.ParseChord(c.Chord)
		if err != nil {
			return err
		}
		e, err := layer.Parse(c.Text)
		if err != nil {
			return err
		}
		l := a.viewed()
		if l == nil {
			return fmt.Errorf("pick a layer first")
		}
		l.Bind[ch] = e
		a.rec = recording{}
		a.save()
		return nil
	},
	"bind.delete": func(a *App, c Cmd) error {
		ch, err := gesture.ParseChord(c.Chord)
		if err != nil {
			return err
		}
		if l := a.viewed(); l != nil {
			delete(l.Bind, ch)
			a.save()
		}
		return nil
	},

	// practice texts
	"stories.reload": func(a *App, c Cmd) error {
		user, err := tutor.Load(a.opt.StoriesDir)
		a.opt.Stories = append(slices.Clone(tutor.Builtin), user...)
		a.tut = tut{drill: -1}
		a.notice = fmt.Sprintf("Loaded %d texts from %s.", len(user), a.opt.StoriesDir)
		return err
	},
	"words.add": func(a *App, c Cmd) error { return a.addWords() },
}

func (a *App) viewed() *layer.Layer {
	if a.view == "global" {
		return &a.prof.Layers.Global
	}
	l, _ := a.prof.Layers.Find(a.view)
	return l
}

// editKit changes the kit. A different set of strikers needs calibration again.
func (a *App) editKit(edit func(gesture.Kit) (gesture.Kit, error)) error {
	k, err := edit(a.prof.Kit)
	if err != nil {
		return err
	}
	changed := len(k.Voices()) != len(a.prof.Kit.Voices())
	a.prof.Kit = k
	if changed {
		a.prof.Calibrated, a.cal = false, cal{}
		if set, added, err := a.prof.Layers.WithMissing(k); err == nil && len(added) > 0 {
			a.prof.Layers = set
		}
	}
	a.save()
	return nil
}

// missingWords are words of the practice texts without an english chord.
func (a *App) missingWords() []string {
	en, ok := a.prof.Layers.Find("english")
	if !ok {
		return nil
	}
	have := map[string]bool{}
	for _, e := range en.Bind {
		have[strings.ToLower(strings.TrimSpace(e.Src()))] = true
	}
	var out []string
	for _, w := range tutor.Words(a.opt.Stories) {
		w = strings.ToLower(strings.Trim(w, ".,!?;:\"()"))
		if w != "" && !have[w] && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

func (a *App) addWords() error {
	en, ok := a.prof.Layers.Find("english")
	if !ok {
		return fmt.Errorf("no english layer")
	}
	added := 0
	for _, w := range a.missingWords() {
		ch, ok := a.prof.Layers.Unused(a.prof.Kit)
		if !ok {
			break
		}
		e, err := layer.Parse(w + " ")
		if err != nil {
			return err
		}
		en.Bind[ch] = e
		added++
	}
	a.notice = fmt.Sprintf("Gave %d words their own chord.", added)
	a.save()
	return nil
}
