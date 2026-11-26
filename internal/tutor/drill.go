package tutor

import (
	"fmt"

	"coinchord/internal/gesture"
	"coinchord/internal/layer"
)

// Drill is a host-skill lesson: each step is one action, not text.
type Drill struct {
	Title, Intro, Layer string
	Steps               []DrillStep
}

// DrillStep asks for the binding whose source is Want.
type DrillStep struct{ Say, Want string }

// Drills teach host control. None of them closes windows or tabs, so a
// practice run with output on loses nothing.
var Drills = []Drill{
	{"Windows", "Move between windows and desktops without touching the mouse.", "window", []DrillStep{
		{"Switch to the next window", "{do:switch}"}, {"Switch back", "{do:switch-back}"},
		{"Maximize the window", "{do:maximize}"}, {"Snap it left", "{do:snap-left}"}, {"Snap it right", "{do:snap-right}"},
		{"Next desktop", "{do:desktop-next}"}, {"Previous desktop", "{do:desktop-prev}"},
		{"Show the desktop", "{do:show-desktop}"}, {"Show it again", "{do:show-desktop}"}, {"Open the launcher", "{do:launcher}"},
		{"Close the launcher", "{esc}"},
	}},
	{"Navigation", "Move the text cursor: left hand goes left and up, right hand goes right and down.", "nav", []DrillStep{
		{"Cursor right", "{right}"}, {"Cursor left", "{left}"}, {"Line down", "{down}"}, {"Line up", "{up}"},
		{"Word right", "{do:word-right}"}, {"Word left", "{do:word-left}"}, {"End of line", "{end}"}, {"Start of line", "{home}"},
		{"Select one character", "{shift+right}"}, {"Copy it", "{do:copy}"}, {"Go to the end", "{do:doc-end}"},
		{"Paste", "{do:paste}"}, {"Undo", "{do:undo}"}, {"Go to the start", "{do:doc-start}"},
	}},
	{"Scrolling", "Circles scroll: left hand up, right hand down. Curves jump a page.", "nav", []DrillStep{
		{"Scroll down", "{scroll:3}"}, {"Scroll down again", "{scroll:3}"}, {"Scroll up", "{scroll:-3}"},
		{"Page down", "{pgdn}"}, {"Page up", "{pgup}"}, {"Jump to the end", "{do:doc-end}"}, {"Back to the top", "{do:doc-start}"},
	}},
	{"Mouse pad", "Drive the pointer: taps nudge, slides and curves jump, knocks click.", "mouse", []DrillStep{
		{"Nudge right", "{move:15,0}"}, {"Nudge left", "{move:-15,0}"}, {"Nudge down", "{move:0,15}"}, {"Nudge up", "{move:0,-15}"},
		{"Jump right", "{move:150,0}"}, {"Jump down", "{move:0,150}"}, {"Jump left", "{move:-150,0}"}, {"Jump up", "{move:0,-150}"},
		{"Click", "{click:left}"}, {"Double-click", "{double:left}"}, {"Right-click", "{click:right}"}, {"Close the menu", "{esc}"},
		{"Start a drag", "{down:left}"}, {"Drag right", "{move:150,0}"}, {"Drop", "{up:left}"}, {"Scroll down", "{scroll:3}"},
	}},
	{"Tabs & browser", "Tabs and history: circles step through tabs.", "window", []DrillStep{
		{"Open a new tab", "{do:tab-new}"}, {"Previous tab", "{do:tab-prev}"}, {"Next tab", "{do:tab-next}"},
		{"Go back", "{do:back}"}, {"Go forward", "{do:forward}"}, {"Full screen", "{do:fullscreen}"}, {"Leave full screen", "{do:fullscreen}"},
	}},
}

// PlanDrill finds the chords of a drill, starting in layer start. It
// prefers the active layer, then the drill layer, then any layer, and
// switches layers as needed. It ends back in start.
func PlanDrill(s *layer.Set, k gesture.Kit, start string, d Drill) ([]Step, error) {
	p := newPlanner(s, k)
	active := start
	var steps []Step
	find := func(name, want string) (gesture.Chord, layer.Expansion, bool) {
		l, ok := s.Find(name)
		if !ok {
			return gesture.Chord{}, layer.Expansion{}, false
		}
		for _, c := range p.order[name] {
			if l.Bind[c].Src() == want {
				return c, l.Bind[c], true
			}
		}
		return gesture.Chord{}, layer.Expansion{}, false
	}
	for _, ds := range d.Steps {
		names := []string{active, d.Layer}
		for _, l := range s.Layers {
			names = append(names, l.Name)
		}
		found := false
		for _, name := range names {
			c, e, ok := find(name, ds.Want)
			if !ok {
				continue
			}
			if name != active {
				sc, se, ok := p.switchTo(name)
				if !ok {
					continue
				}
				steps = append(steps, Step{Chord: sc, Layer: active, Expand: se, Say: "Switch to the " + name + " layer"})
				active = name
			}
			steps = append(steps, Step{Chord: c, Layer: active, Expand: e, Say: ds.Say})
			found = true
			break
		}
		if !found {
			return steps, fmt.Errorf("no binding for %s (%s)", ds.Want, ds.Say)
		}
	}
	if active != start {
		if c, e, ok := p.switchTo(start); ok {
			steps = append(steps, Step{Chord: c, Layer: active, Expand: e, Say: "Back to the " + start + " layer"})
		}
	}
	return steps, nil
}
