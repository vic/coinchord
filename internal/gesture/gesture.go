// Package gesture defines the gesture vocabulary and chords built from it.
//
// A striker is one sound source held or worn by one hand: a coin, a key,
// a fingernail. A kit is the set of strikers a person calibrated.
package gesture

import (
	"fmt"
	"strconv"
	"strings"
)

// Hand that holds a striker.
type Hand uint8

const (
	Left Hand = iota
	Right
)

func (h Hand) String() string { return [...]string{"L", "R"}[h] }

// Word is the human name of the hand.
func (h Hand) Word() string { return [...]string{"Left", "Right"}[h] }

// MaxSlot is the highest numbered slot per hand.
const MaxSlot = 99

// Striker is one sound source: a hand and a slot. Slot 0 is the single
// object of that hand ("L"); slots 1..MaxSlot are numbered ("L3").
// Build it with NewStriker or ParseStriker; both reject other slots.
type Striker struct {
	hand Hand
	slot uint8
}

// NewStriker builds a striker; slot must be 0..MaxSlot.
func NewStriker(h Hand, slot int) (Striker, error) {
	if slot < 0 || slot > MaxSlot || h > Right {
		return Striker{}, fmt.Errorf("striker %v%d: slot must be 0..%d", h, slot, MaxSlot)
	}
	return Striker{h, uint8(slot)}, nil
}

// Hand returns the hand of the striker.
func (s Striker) Hand() Hand { return s.hand }

func (s Striker) String() string {
	if s.slot == 0 {
		return s.hand.String()
	}
	return fmt.Sprintf("%v%d", s.hand, s.slot)
}

// ParseStriker parses "L", "R", "L1".."R5".
func ParseStriker(s string) (Striker, error) {
	if s == "" || (s[0] != 'L' && s[0] != 'R') {
		return Striker{}, fmt.Errorf("striker %q: want L, R, or L1..R%d", s, MaxSlot)
	}
	h := map[byte]Hand{'L': Left, 'R': Right}[s[0]]
	if len(s) == 1 {
		return Striker{h, 0}, nil
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil || n < 1 || n > MaxSlot || s[1] == '0' {
		return Striker{}, fmt.Errorf("striker %q: slot must be 1..%d", s, MaxSlot)
	}
	return Striker{h, uint8(n)}, nil
}

// Source is what sounded in one stroke: one striker, or two strikers of
// different hands at the same time. Build it with Solo or Duo.
type Source struct {
	a, b Striker
	duo  bool
}

// Solo is one striker alone.
func Solo(s Striker) Source { return Source{a: s} }

// Duo is two strikers of different hands sounding together.
// The left striker always comes first, so Duo(x,y) == Duo(y,x).
func Duo(x, y Striker) (Source, error) {
	if x.hand == y.hand {
		return Source{}, fmt.Errorf("duo %v&%v: strikers must be on different hands", x, y)
	}
	if x.hand == Right {
		x, y = y, x
	}
	return Source{x, y, true}, nil
}

// Strikers returns the one or two strikers of the source.
func (s Source) Strikers() []Striker {
	if s.duo {
		return []Striker{s.a, s.b}
	}
	return []Striker{s.a}
}

// IsDuo reports whether two strikers sounded together.
func (s Source) IsDuo() bool { return s.duo }

func (s Source) String() string {
	if s.duo {
		return s.a.String() + "&" + s.b.String()
	}
	return s.a.String()
}

// ParseSource parses "L", "L2", "L&R", "L1&R3".
func ParseSource(s string) (Source, error) {
	x, y, duo := strings.Cut(s, "&")
	a, err := ParseStriker(x)
	if err != nil || !duo {
		return Solo(a), err
	}
	b, err := ParseStriker(y)
	if err != nil {
		return Source{}, err
	}
	return Duo(a, b)
}

// Shape is the geometry drawn on the desk.
type Shape uint8

const (
	Tap Shape = iota
	Line
	Curve
	Circle
	Knock
	Slide
	Zigzag
	shapeCount
)

// Shapes in ease order: easiest first.
var Shapes = [shapeCount]Shape{Tap, Line, Curve, Circle, Knock, Slide, Zigzag}

var shapeNames = [shapeCount]string{"TAP", "LINE", "CURV", "CIRC", "KNOCK", "SLIDE", "ZIG"}

var glyphs = [shapeCount]string{"•", "─", "╮", "○", "✱", "═", "≈"}

var howTo = [shapeCount]string{
	"A light, quick tap straight down. Lift right away.",
	"Drag about 1 inch in a straight line, even pressure.",
	"Drag and hook the wrist at the end, like a letter J.",
	"Draw a small circle, the size of a dime, 1.5 turns.",
	"A firm tap, about twice as loud as a normal tap.",
	"A long straight drag, 2 inches or more, slow and steady.",
	"Scribble fast back and forth, 3 or more quick zigs.",
}

func (s Shape) String() string { return shapeNames[s] }

// Glyph returns a one-cell symbol of the shape.
func (s Shape) Glyph() string { return glyphs[s] }

// HowTo explains how to perform the shape.
func (s Shape) HowTo() string { return howTo[s] }

func parseShape(s string) (Shape, error) {
	for i, n := range shapeNames {
		if n == s {
			return Shape(i), nil
		}
	}
	return 0, fmt.Errorf("shape %q: want one of %v", s, shapeNames)
}

// Gesture is one stroke: a source and a shape.
type Gesture struct {
	Src   Source
	Shape Shape
}

func (g Gesture) String() string { return g.Src.String() + "_" + g.Shape.String() }

// ParseGesture parses names such as "L_TAP", "L2&R1_CIRC".
func ParseGesture(s string) (Gesture, error) {
	src, sh, ok := strings.Cut(s, "_")
	if !ok {
		return Gesture{}, fmt.Errorf("gesture %q: want SOURCE_SHAPE", s)
	}
	a, err := ParseSource(src)
	if err != nil {
		return Gesture{}, err
	}
	b, err := parseShape(sh)
	return Gesture{a, b}, err
}

// Chord is one or two gestures performed inside the chord window.
// Build chords with Single or Pair.
type Chord struct {
	first, second Gesture
	pair          bool
}

// Single is a one-gesture chord.
func Single(g Gesture) Chord { return Chord{first: g} }

// Pair is a two-gesture chord.
func Pair(a, b Gesture) Chord { return Chord{first: a, second: b, pair: true} }

// Steps returns the gestures of the chord in order.
func (c Chord) Steps() []Gesture {
	if c.pair {
		return []Gesture{c.first, c.second}
	}
	return []Gesture{c.first}
}

func (c Chord) String() string {
	if c.pair {
		return c.first.String() + "+" + c.second.String()
	}
	return c.first.String()
}

// ParseChord parses "L_TAP" or "L_TAP+R_LINE".
func ParseChord(s string) (Chord, error) {
	a, b, two := strings.Cut(strings.TrimSpace(s), "+")
	ga, err := ParseGesture(a)
	if err != nil || !two {
		return Single(ga), err
	}
	gb, err := ParseGesture(b)
	return Pair(ga, gb), err
}

func (c Chord) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

func (c *Chord) UnmarshalText(b []byte) error {
	p, err := ParseChord(string(b))
	*c = p
	return err
}

func (s Striker) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

func (s *Striker) UnmarshalText(b []byte) error {
	p, err := ParseStriker(string(b))
	*s = p
	return err
}
