//go:build darwin

package inject

import (
	"fmt"
	"time"
	"unicode/utf16"

	"github.com/ebitengine/purego"

	"coinchord/internal/layer"
)

// CoreGraphics constants from CGEventTypes.h and Events.h.
const (
	stateHID   = 1 // kCGEventSourceStateHIDSystemState
	tapHID     = 0 // kCGHIDEventTap
	flagShift  = 0x00020000
	flagCtrl   = 0x00040000
	flagAlt    = 0x00080000
	flagCmd    = 0x00100000
	scrollLine = 1  // kCGScrollEventUnitLine
	clickState = 1  // kCGMouseEventClickState
	chunk      = 20 // CoreGraphics types at most 20 UTF-16 units per event
)

type cgPoint struct{ X, Y float64 }

var (
	evSource     func(state int32) uintptr
	evKeyboard   func(src uintptr, key uint16, down bool) uintptr
	evSetFlags   func(ev uintptr, flags uint64)
	evSetUnicode func(ev uintptr, n uint64, s *uint16)
	evPost       func(tap uint32, ev uintptr)
	evCreate     func(src uintptr) uintptr
	evLocation   func(ev uintptr) cgPoint
	evMouse      func(src uintptr, typ uint32, at cgPoint, button uint32) uintptr
	evScroll     func(src uintptr, units, count uint32, w1, w2, w3 int32) uintptr
	evSetInt     func(ev uintptr, field uint32, v int64)
	cfRelease    func(ref uintptr)
)

var macKeys = map[string]uint16{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9, "b": 11, "q": 12, "w": 13, "e": 14,
	"r": 15, "y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23, "=": 24, "9": 25, "7": 26, "-": 27,
	"8": 28, "0": 29, "]": 30, "o": 31, "u": 32, "[": 33, "i": 34, "p": 35, "l": 37, "j": 38, "'": 39, "k": 40,
	";": 41, "\\": 42, ",": 43, "/": 44, "n": 45, "m": 46, ".": 47, "`": 50,
	"enter": 36, "tab": 48, "space": 49, "bs": 51, "esc": 53, "del": 117,
	"home": 115, "pgup": 116, "end": 119, "pgdn": 121, "left": 123, "right": 124, "down": 125, "up": 126,
	"f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97, "f7": 98, "f8": 100, "f9": 101, "f10": 109, "f11": 103, "f12": 111,
}

// mouse event types per button: down, up, dragged
var mouseTypes = map[layer.Button][3]uint32{
	layer.LeftButton:   {1, 2, 6},
	layer.RightButton:  {3, 4, 7},
	layer.MiddleButton: {25, 26, 27},
}

type mac struct {
	src  uintptr
	held *layer.Button // button down during a drag
}

func load(lib uintptr, fn any, name string) error {
	sym, err := purego.Dlsym(lib, name)
	if err != nil {
		return fmt.Errorf("CoreGraphics: %s: %w", name, err)
	}
	purego.RegisterFunc(fn, sym)
	return nil
}

// Open loads CoreGraphics. macOS asks once for Accessibility permission
// (System Settings → Privacy & Security → Accessibility) for the terminal.
func Open() (Device, error) {
	cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	for _, f := range []struct {
		lib  uintptr
		fn   any
		name string
	}{
		{cg, &evSource, "CGEventSourceCreate"}, {cg, &evKeyboard, "CGEventCreateKeyboardEvent"},
		{cg, &evSetFlags, "CGEventSetFlags"}, {cg, &evSetUnicode, "CGEventKeyboardSetUnicodeString"},
		{cg, &evPost, "CGEventPost"}, {cg, &evCreate, "CGEventCreate"}, {cg, &evLocation, "CGEventGetLocation"},
		{cg, &evMouse, "CGEventCreateMouseEvent"}, {cg, &evScroll, "CGEventCreateScrollWheelEvent2"},
		{cg, &evSetInt, "CGEventSetIntegerValueField"}, {cf, &cfRelease, "CFRelease"},
	} {
		if err := load(f.lib, f.fn, f.name); err != nil {
			return nil, err
		}
	}
	return &mac{src: evSource(stateHID)}, nil
}

func post(ev uintptr) error {
	if ev == 0 {
		return fmt.Errorf("CoreGraphics refused the event (Accessibility permission?)")
	}
	evPost(tapHID, ev)
	cfRelease(ev)
	return nil
}

func (m *mac) Text(s string) error {
	units := utf16.Encode([]rune(s))
	for len(units) > 0 {
		n := min(chunk, len(units))
		for _, down := range []bool{true, false} {
			ev := evKeyboard(m.src, 0, down)
			if ev != 0 {
				evSetUnicode(ev, uint64(n), &units[0])
			}
			if err := post(ev); err != nil {
				return err
			}
		}
		units = units[n:]
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}

func (m *mac) Combo(c layer.Combo) error {
	code, ok := macKeys[c.Key]
	var flags uint64
	if !ok {
		return fmt.Errorf("no mac key for %q", c.Key)
	}
	for _, f := range []struct {
		m    layer.Mod
		flag uint64
	}{{layer.Ctrl, flagCtrl}, {layer.Alt, flagAlt}, {layer.Shift, flagShift}, {layer.Super, flagCmd}} {
		if c.Mods&f.m != 0 {
			flags |= f.flag
		}
	}
	for _, down := range []bool{true, false} {
		ev := evKeyboard(m.src, code, down)
		if ev != 0 {
			evSetFlags(ev, flags)
		}
		if err := post(ev); err != nil {
			return err
		}
	}
	return nil
}

func (m *mac) at() cgPoint {
	ev := evCreate(0)
	if ev == 0 {
		return cgPoint{}
	}
	defer cfRelease(ev)
	return evLocation(ev)
}

func (m *mac) click(b layer.Button, count int64) error {
	p := m.at()
	for _, i := range []int{0, 1} {
		ev := evMouse(m.src, mouseTypes[b][i], p, uint32(b))
		if ev != 0 {
			evSetInt(ev, clickState, count)
		}
		if err := post(ev); err != nil {
			return err
		}
	}
	return nil
}

func (m *mac) Mouse(a layer.Mouse) error {
	switch a.Kind {
	case layer.Move:
		p := m.at()
		p.X, p.Y = p.X+float64(a.DX), p.Y+float64(a.DY)
		typ, b := uint32(5), layer.LeftButton // kCGEventMouseMoved
		if m.held != nil {
			typ, b = mouseTypes[*m.held][2], *m.held
		}
		return post(evMouse(m.src, typ, p, uint32(b)))
	case layer.Scroll: // positive wheel = up; our DY > 0 means down
		return post(evScroll(m.src, scrollLine, 2, int32(-a.DY), int32(-a.DX), 0))
	case layer.Down:
		b := a.Button
		m.held = &b
		return post(evMouse(m.src, mouseTypes[b][0], m.at(), uint32(b)))
	case layer.Up:
		m.held = nil
		return post(evMouse(m.src, mouseTypes[a.Button][1], m.at(), uint32(a.Button)))
	case layer.Double:
		if err := m.click(a.Button, 1); err != nil {
			return err
		}
		return m.click(a.Button, 2)
	}
	return m.click(a.Button, 1)
}

func (m *mac) Close() error {
	if m.src != 0 {
		cfRelease(m.src)
	}
	return nil
}
