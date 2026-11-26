package layer

import (
	"encoding/json"
	"testing"

	"coinchord/internal/kits"
)

func TestParse(t *testing.T) {
	good := map[string]int{"hi": 1, "{enter}": 1, "a{ctrl+alt+left}b": 3, "{layer:kbd}": 1, "{undo}": 1, "{{x": 1, "x{tab}": 2}
	for src, n := range good {
		e, err := Parse(src)
		if err != nil || len(e.Actions()) != n {
			t.Errorf("%q: %v actions=%d want %d", src, err, len(e.Actions()), n)
		}
	}
	for _, bad := range []string{"", "{", "{nope}", "{ctrl+}", "{hyper+a}", "{layer:}"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestTypist(t *testing.T) {
	var ty Typist
	for _, s := range []string{"once upon a time, ", "a ", "fox ", ". ", "the ", "end.", "{undo}", "end", "{bs}", "d!"} {
		e, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		ty.Apply(e)
	}
	if got, want := ty.Text(), "Once upon a time, a fox. The end!"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultsAllKits(t *testing.T) {
	for _, p := range kits.Presets {
		k, err := p.Kit()
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		s, err := Defaults(k, []string{"fox", "Owl."})
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		if len(s.Global.Bind) != len(builtin) {
			t.Fatalf("%s: %d switches", p.Name, len(s.Global.Bind))
		}
		for _, l := range s.Layers {
			for c := range l.Bind {
				if _, clash := s.Global.Bind[c]; clash {
					t.Fatalf("%s/%s: %v shadowed by a switch", p.Name, l.Name, c)
				}
				if !Reachable(k, c) {
					t.Fatalf("%s/%s: %v not in kit", p.Name, l.Name, c)
				}
			}
		}
		b, _ := json.Marshal(s)
		var back Set
		if err := json.Unmarshal(b, &back); err != nil || len(back.Layers) != len(s.Layers) {
			t.Fatalf("%s: json roundtrip %v", p.Name, err)
		}
	}
}
