package inject

import (
	"strings"
	"testing"

	"coinchord/internal/layer"
)

// fake records what a backend was asked to do.
type fake struct{ log []string }

func (f *fake) Text(s string) error       { f.log = append(f.log, "text:"+s); return nil }
func (f *fake) Combo(c layer.Combo) error { f.log = append(f.log, "key:"+c.String()); return nil }
func (f *fake) Mouse(m layer.Mouse) error { f.log = append(f.log, "mouse:"+m.String()); return nil }
func (f *fake) Close() error              { return nil }

func run(t *testing.T, goos string, srcs ...string) string {
	t.Helper()
	f := &fake{}
	h := New(f, goos)
	for _, s := range srcs {
		e, err := layer.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := h.Run(e); err != nil {
			t.Fatal(err)
		}
	}
	return strings.Join(f.log, " ")
}

func TestHostRun(t *testing.T) {
	cases := []struct {
		goos string
		srcs []string
		want string
	}{
		{"linux", []string{"a\tb\n"}, "text:a key:tab text:b key:enter"},
		{"linux", []string{"{do:switch}"}, "key:alt+tab"},
		{"darwin", []string{"{do:copy}"}, "key:super+c"},
		{"windows", []string{"{do:desktop-next}"}, "key:ctrl+super+right"},
		{"linux", []string{"{scroll:-3}{click:left}"}, "mouse:scroll:0,-3 mouse:click:left"},
		{"linux", []string{"héllo ", "{undo}"}, "text:héllo  key:bs key:bs key:bs key:bs key:bs key:bs"},
	}
	for _, c := range cases {
		if got := run(t, c.goos, c.srcs...); got != c.want {
			t.Errorf("%s %v:\n got %s\nwant %s", c.goos, c.srcs, got, c.want)
		}
	}
	f := &fake{}
	e, _ := layer.Parse("{do:snap-left}")
	if err := New(f, "darwin").Run(e); err == nil {
		t.Fatal("mac has no snap shortcut: want error")
	}
}
