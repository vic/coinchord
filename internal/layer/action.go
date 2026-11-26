// Package layer maps chords to expansions and applies them to text.
package layer

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// Action is one effect of an expansion: Text, Combo, Switch, Undo, Mouse or Command.
type Action interface{ act() }

// Text types literal text.
type Text string

// Mod is a set of modifier keys.
type Mod uint8

const (
	Ctrl Mod = 1 << iota
	Alt
	Shift
	Super
)

var modNames = []string{"ctrl", "alt", "shift", "super"}

// Keys are the named keys a combo can press, besides one printable character.
var Keys = []string{
	"enter", "bs", "tab", "space", "esc", "del", "left", "right", "up", "down",
	"home", "end", "pgup", "pgdn", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12",
}

// Combo presses a key with modifiers. Build it with parseCombo.
type Combo struct {
	Mods Mod
	Key  string
}

// Switch makes another layer active.
type Switch string

// Undo removes the effect of the previous expansion.
type Undo struct{}

func (Text) act()   {}
func (Combo) act()  {}
func (Switch) act() {}
func (Undo) act()   {}

func (c Combo) String() string {
	var parts []string
	for i, n := range modNames {
		if c.Mods&(1<<i) != 0 {
			parts = append(parts, n)
		}
	}
	return strings.Join(append(parts, c.Key), "+")
}

func parseCombo(s string) (Combo, error) {
	parts := strings.Split(s, "+")
	var c Combo
	for _, p := range parts[:len(parts)-1] {
		i := slices.Index(modNames, p)
		if i < 0 {
			return Combo{}, fmt.Errorf("unknown modifier %q in {%s}", p, s)
		}
		c.Mods |= 1 << i
	}
	c.Key = parts[len(parts)-1]
	if !slices.Contains(Keys, c.Key) && utf8.RuneCountInString(c.Key) != 1 {
		return Combo{}, fmt.Errorf("unknown key %q in {%s}", c.Key, s)
	}
	return c, nil
}

// Expansion is the parsed form of a binding source such as
// "hello{enter}", "{ctrl+alt+left}", "{layer:kbd}", "{undo}", "{{".
type Expansion struct {
	src  string
	acts []Action
}

// Parse builds an expansion. Unknown tokens are errors.
func Parse(src string) (Expansion, error) {
	var acts []Action
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			acts = append(acts, Text(text.String()))
			text.Reset()
		}
	}
	for rest := src; rest != ""; {
		switch {
		case strings.HasPrefix(rest, "{{"):
			text.WriteByte('{')
			rest = rest[2:]
		case rest[0] == '{':
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return Expansion{}, fmt.Errorf("missing } in %q", src)
			}
			tok := rest[1:end]
			rest = rest[end+1:]
			flush()
			a, err := token(tok)
			if err != nil {
				return Expansion{}, err
			}
			acts = append(acts, a)
		default:
			r, n := utf8.DecodeRuneInString(rest)
			text.WriteRune(r)
			rest = rest[n:]
		}
	}
	flush()
	if len(acts) == 0 {
		return Expansion{}, fmt.Errorf("empty expansion")
	}
	return Expansion{src, acts}, nil
}

func token(tok string) (Action, error) {
	if name, ok := strings.CutPrefix(tok, "layer:"); ok && name != "" {
		return Switch(name), nil
	}
	if tok == "undo" {
		return Undo{}, nil
	}
	if name, ok := strings.CutPrefix(tok, "do:"); ok {
		return parseCommand(name)
	}
	if kind, arg, ok := strings.Cut(tok, ":"); ok && slices.Contains(mouseNames, kind) {
		return parseMouse(kind, arg)
	}
	return parseCombo(tok)
}

// Src returns the source text of the expansion.
func (e Expansion) Src() string { return e.src }

// Actions returns the parsed actions.
func (e Expansion) Actions() []Action { return slices.Clone(e.acts) }

// Show is a one-line preview for lists.
func (e Expansion) Show() string {
	return strings.NewReplacer("\n", "⏎", "\t", "⇥", " ", "·").Replace(e.src)
}

func (e Expansion) MarshalText() ([]byte, error) { return []byte(e.src), nil }

func (e *Expansion) UnmarshalText(b []byte) error {
	p, err := Parse(string(b))
	*e = p
	return err
}
