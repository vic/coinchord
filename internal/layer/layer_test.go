package layer

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"coinchord/internal/gesture"
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

func TestHostActions(t *testing.T) {
	good := map[string]string{"{move:-40,25}": "move:-40,25", "{scroll:3}": "scroll:0,3", "{click:right}": "click:right", "{down:left}": "down:left"}
	for src, want := range good {
		e, err := Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if m, ok := e.Actions()[0].(Mouse); !ok || m.String() != want {
			t.Fatalf("%s: got %v", src, e.Actions()[0])
		}
	}
	for _, bad := range []string{"{move:1}", "{move:a,b}", "{scroll:99999}", "{click:thumb}", "{do:fly}"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
	// every command has a shortcut on linux and windows; mac lacks only snapping
	for _, c := range Commands() {
		for _, goos := range []string{"linux", "windows", "darwin"} {
			_, err := c.Shortcut(goos)
			if want := goos == "darwin" && (c == CmdSnapLeft || c == CmdSnapRight); (err != nil) != want {
				t.Errorf("%s on %s: %v", c, goos, err)
			}
		}
	}
	var ty Typist
	e, _ := Parse("hi{do:copy}{click:left}")
	ty.Apply(e)
	if ty.Text() != "Hi" || len(ty.Host) != 2 {
		t.Fatalf("typist %q %v", ty.Text(), ty.Host)
	}
}

func TestHostLayerPins(t *testing.T) {
	for _, c := range []struct {
		preset int
		left   string
	}{{0, "L"}, {5, "L1"}} {
		k, _ := kits.Presets[c.preset].Kit()
		s, err := Defaults(k, nil)
		if err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]string{"nav": "{left}", "window": "{do:switch-back}", "mouse": "{move:-15,0}"} {
			l, _ := s.Find(name)
			ch, err := gesture.ParseChord(c.left + "_TAP")
			if err != nil {
				t.Fatal(err)
			}
			if got := l.Bind[ch].Src(); got != want {
				t.Errorf("%s %s %s_TAP = %q want %q", kits.Presets[c.preset].Name, name, c.left, got, want)
			}
		}
	}
}

func TestWithMissing(t *testing.T) {
	k, _ := kits.Presets[0].Kit()
	s, _ := Defaults(k, nil)
	en, _ := s.Find("english")
	mine := gesture.Single(k.Gestures()[0])
	en.Bind[mine] = Expansion{src: "custom", acts: []Action{Text("custom")}}
	old := Set{Global: s.Global, Layers: []Layer{*en}}
	got, added, err := old.WithMissing(k)
	if err != nil || len(added) != len(builtin)-1 {
		t.Fatalf("added %v %v", added, err)
	}
	if e, _ := got.Find("english"); e.Bind[mine].Src() != "custom" {
		t.Fatal("user binding changed")
	}
	if _, ok := got.Find("mouse"); !ok {
		t.Fatal("mouse layer missing")
	}
	for _, name := range added {
		if !strings.Contains(fmt.Sprint(got.Global.Bind), "{layer:"+name+"}") {
			t.Fatalf("no switch for %s", name)
		}
	}
}
