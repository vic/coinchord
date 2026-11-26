package tutor

import (
	"strings"
	"testing"
	"time"

	"coinchord/internal/kits"
	"coinchord/internal/layer"
)

// Every built-in story must be typable with the default layers of every kit,
// and replaying the planned chords must type the sentence.
func TestPlanBuiltinStories(t *testing.T) {
	extra := Story{"user", "Zyx paid 42 coins; wow!"}
	for _, p := range kits.Presets {
		k, err := p.Kit()
		if err != nil {
			t.Fatal(err)
		}
		set, err := layer.Defaults(k, Words(Builtin))
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range append(Builtin, extra) {
			for _, sent := range st.Sentences() {
				steps, err := Plan(&set, k, "english", sent)
				if err != nil {
					for _, s := range steps[:min(12, len(steps))] {
						t.Logf("%s %v %q -> %q", s.Layer, s.Chord, s.Expand.Src(), s.Text)
					}
					t.Fatalf("%s/%s: %v", p.Name, st.Title, err)
				}
				ty := layer.Typist{Active: "english"}
				for _, s := range steps {
					e, ok := set.Lookup(ty.Active, s.Chord)
					if !ok || e.Src() != s.Expand.Src() {
						t.Fatalf("%s: chord %v not bound in %s", sent, s.Chord, ty.Active)
					}
					ty.Apply(e)
				}
				if !done(ty.Text(), sent) || ty.Active != "english" {
					t.Fatalf("%s: typed %q in %s", sent, ty.Text(), ty.Active)
				}
			}
		}
	}
}

func TestSentences(t *testing.T) {
	got := Story{"", "One. Two!  Three? four"}.Sentences()
	want := []string{"One. ", "Two! ", "Three? ", "four "}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
}

func TestStats(t *testing.T) {
	t0 := time.Unix(0, 0)
	s := NewStats(t0)
	s.Hit(30, t0.Add(30*time.Second))
	s.Hit(30, t0.Add(60*time.Second))
	if s.WPM() != 12 || s.Accuracy() != 1 || s.CPM() != 2 {
		t.Fatalf("wpm %v acc %v cpm %v", s.WPM(), s.Accuracy(), s.CPM())
	}
}

// Every drill must be playable with the default layers of every kit, and
// replaying it must perform exactly the asked actions, ending in english.
func TestPlanDrills(t *testing.T) {
	for _, p := range kits.Presets {
		k, _ := p.Kit()
		set, err := layer.Defaults(k, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range Drills {
			steps, err := PlanDrill(&set, k, "english", d)
			if err != nil {
				t.Fatalf("%s/%s: %v", p.Name, d.Title, err)
			}
			ty := layer.Typist{Active: "english"}
			var did []string
			for _, s := range steps {
				e, ok := set.Lookup(ty.Active, s.Chord)
				if !ok || e.Src() != s.Expand.Src() {
					t.Fatalf("%s/%s: %v not bound in %s", p.Name, d.Title, s.Chord, ty.Active)
				}
				if _, sw := e.Actions()[0].(layer.Switch); !sw {
					did = append(did, e.Src())
				}
				ty.Apply(e)
			}
			if len(did) != len(d.Steps) || ty.Active != "english" {
				t.Fatalf("%s/%s: did %d of %d, ends in %s", p.Name, d.Title, len(did), len(d.Steps), ty.Active)
			}
			for i, ds := range d.Steps {
				if did[i] != ds.Want {
					t.Fatalf("%s/%s step %d: did %s want %s", p.Name, d.Title, i, did[i], ds.Want)
				}
			}
		}
	}
}
