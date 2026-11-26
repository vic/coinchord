//go:build linux

package inject

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"coinchord/internal/layer"
)

// Linux input constants from <linux/input-event-codes.h> and <linux/uinput.h>.
const (
	evSyn = 0x00
	evKey = 0x01
	evRel = 0x02

	relX      = 0x00
	relY      = 0x01
	relHWheel = 0x06
	relWheel  = 0x08

	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112

	keyLeftCtrl  = 29
	keyLeftShift = 42
	keyLeftAlt   = 56
	keyLeftMeta  = 125

	uiSetEvBit   = 0x40045564 // _IOW('U', 100, int)
	uiSetKeyBit  = 0x40045565 // _IOW('U', 101, int)
	uiSetRelBit  = 0x40045566 // _IOW('U', 102, int)
	uiDevSetup   = 0x405c5503 // _IOW('U', 3, struct uinput_setup), 92 bytes
	uiDevCreate  = 0x5501     // _IO('U', 1)
	uiDevDestroy = 0x5502     // _IO('U', 2)

	busVirtual = 0x06
	keyGap     = 2 * time.Millisecond // apps drop keys that arrive faster
)

// usKeys maps printable ASCII to US-layout key codes; shifted runes set shift.
var usKeys = func() map[rune]struct {
	code  uint16
	shift bool
} {
	m := map[rune]struct {
		code  uint16
		shift bool
	}{}
	rows := []struct {
		plain, shifted string
		first          uint16
	}{
		{"1234567890-=", "!@#$%^&*()_+", 2},
		{"qwertyuiop[]", "QWERTYUIOP{}", 16},
		{"asdfghjkl;'`", "ASDFGHJKL:\"~", 30},
		{"\\zxcvbnm,./", "|ZXCVBNM<>?", 43},
	}
	for _, r := range rows {
		for i, c := range r.plain {
			m[c] = struct {
				code  uint16
				shift bool
			}{r.first + uint16(i), false}
		}
		for i, c := range r.shifted {
			m[c] = struct {
				code  uint16
				shift bool
			}{r.first + uint16(i), true}
		}
	}
	m[' '] = struct {
		code  uint16
		shift bool
	}{57, false}
	return m
}()

var namedKeys = map[string]uint16{
	"enter": 28, "bs": 14, "tab": 15, "space": 57, "esc": 1, "del": 111,
	"left": 105, "right": 106, "up": 103, "down": 108, "home": 102, "end": 107, "pgup": 104, "pgdn": 109,
	"f1": 59, "f2": 60, "f3": 61, "f4": 62, "f5": 63, "f6": 64, "f7": 65, "f8": 66, "f9": 67, "f10": 68, "f11": 87, "f12": 88,
}

var modKeys = []struct {
	m    layer.Mod
	code uint16
}{{layer.Ctrl, keyLeftCtrl}, {layer.Alt, keyLeftAlt}, {layer.Shift, keyLeftShift}, {layer.Super, keyLeftMeta}}

type event struct {
	typ, code uint16
	value     int32
}

// encode writes events as struct input_event (64-bit: 16-byte timeval, then type, code, value).
func encode(w io.Writer, evs []event) error {
	buf := make([]byte, 0, 24*len(evs))
	for _, e := range evs {
		buf = append(buf, make([]byte, 16)...) // time: the kernel fills it
		buf = binary.NativeEndian.AppendUint16(buf, e.typ)
		buf = binary.NativeEndian.AppendUint16(buf, e.code)
		buf = binary.NativeEndian.AppendUint32(buf, uint32(e.value))
	}
	_, err := w.Write(buf)
	return err
}

var syn = event{evSyn, 0, 0}

// tap presses and releases code with held modifiers.
func tap(mods []uint16, code uint16) []event {
	var evs []event
	for _, m := range mods {
		evs = append(evs, event{evKey, m, 1})
	}
	evs = append(evs, event{evKey, code, 1}, syn, event{evKey, code, 0})
	for i := len(mods) - 1; i >= 0; i-- {
		evs = append(evs, event{evKey, mods[i], 0})
	}
	return append(evs, syn)
}

// comboEvents converts a combo to key events.
func comboEvents(c layer.Combo) ([]event, error) {
	var mods []uint16
	for _, mk := range modKeys {
		if c.Mods&mk.m != 0 {
			mods = append(mods, mk.code)
		}
	}
	if code, ok := namedKeys[c.Key]; ok {
		return tap(mods, code), nil
	}
	r := []rune(c.Key)[0]
	k, ok := usKeys[r]
	if !ok {
		return nil, fmt.Errorf("no key for %q", c.Key)
	}
	if k.shift {
		mods = append(mods, keyLeftShift)
	}
	return tap(mods, k.code), nil
}

// textEvents types s. Runes outside US ASCII use the Ctrl+Shift+U unicode
// entry of GTK and IBus apps.
// shortcut: assumes a US layout and GTK/IBus unicode entry; upgrade with an input-method backend.
func textEvents(s string) []event {
	var evs []event
	for _, r := range s {
		if k, ok := usKeys[r]; ok {
			var mods []uint16
			if k.shift {
				mods = []uint16{keyLeftShift}
			}
			evs = append(evs, tap(mods, k.code)...)
			continue
		}
		evs = append(evs, tap([]uint16{keyLeftCtrl, keyLeftShift}, 22)...) // 22 = KEY_U
		for _, h := range strings.ToLower(fmt.Sprintf("%x", r)) {
			evs = append(evs, tap(nil, usKeys[h].code)...)
		}
		evs = append(evs, tap(nil, namedKeys["space"])...)
	}
	return evs
}

var buttonCodes = map[layer.Button]uint16{layer.LeftButton: btnLeft, layer.RightButton: btnRight, layer.MiddleButton: btnMiddle}

func mouseEvents(m layer.Mouse) []event {
	b := buttonCodes[m.Button]
	switch m.Kind {
	case layer.Move:
		return []event{{evRel, relX, int32(m.DX)}, {evRel, relY, int32(m.DY)}, syn}
	case layer.Scroll: // wheel up is positive; our DY > 0 means down
		return []event{{evRel, relWheel, int32(-m.DY)}, {evRel, relHWheel, int32(m.DX)}, syn}
	case layer.Down:
		return []event{{evKey, b, 1}, syn}
	case layer.Up:
		return []event{{evKey, b, 0}, syn}
	case layer.Double:
		return append(mouseEvents(layer.Mouse{Kind: layer.Click, Button: m.Button}), mouseEvents(layer.Mouse{Kind: layer.Click, Button: m.Button})...)
	}
	return []event{{evKey, b, 1}, syn, {evKey, b, 0}, syn}
}

type uinput struct{ f *os.File }

func ioctl(fd uintptr, req, arg uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); e != 0 {
		return e
	}
	return nil
}

// Open creates a virtual keyboard and mouse. It needs write access to
// /dev/uinput (for example membership in the input group, or a udev rule).
func Open() (Device, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/uinput: %w (add a udev rule: KERNEL==\"uinput\", GROUP=\"input\", MODE=\"0660\", and join the input group)", err)
	}
	fd := f.Fd()
	steps := []struct{ req, arg uintptr }{{uiSetEvBit, evKey}, {uiSetEvBit, evRel}, {uiSetEvBit, evSyn}}
	for code := uintptr(1); code < 256; code++ {
		steps = append(steps, struct{ req, arg uintptr }{uiSetKeyBit, code})
	}
	for _, c := range []uintptr{btnLeft, btnRight, btnMiddle} {
		steps = append(steps, struct{ req, arg uintptr }{uiSetKeyBit, c})
	}
	for _, c := range []uintptr{relX, relY, relWheel, relHWheel} {
		steps = append(steps, struct{ req, arg uintptr }{uiSetRelBit, c})
	}
	for _, s := range steps {
		if err := ioctl(fd, s.req, s.arg); err != nil {
			f.Close()
			return nil, fmt.Errorf("uinput setup: %w", err)
		}
	}
	var setup [92]byte // struct uinput_setup: input_id (4 x u16), name[80], ff_effects_max u32
	binary.NativeEndian.PutUint16(setup[0:], busVirtual)
	binary.NativeEndian.PutUint16(setup[2:], 0x1209) // pid.codes open vendor id
	binary.NativeEndian.PutUint16(setup[4:], 0xc0c0)
	copy(setup[8:88], "CoinChord")
	if err := ioctl(fd, uiDevSetup, uintptr(unsafe.Pointer(&setup))); err != nil {
		f.Close()
		return nil, fmt.Errorf("uinput setup: %w", err)
	}
	if err := ioctl(fd, uiDevCreate, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("uinput create: %w", err)
	}
	time.Sleep(200 * time.Millisecond) // let the desktop see the new device before the first key
	return &uinput{f}, nil
}

func (u *uinput) send(evs []event) error {
	// one write per syn report keeps the gap between keys
	start := 0
	for i, e := range evs {
		if e == syn {
			if err := encode(u.f, evs[start:i+1]); err != nil {
				return err
			}
			start = i + 1
			time.Sleep(keyGap)
		}
	}
	return nil
}

func (u *uinput) Text(s string) error { return u.send(textEvents(s)) }

func (u *uinput) Combo(c layer.Combo) error {
	evs, err := comboEvents(c)
	if err != nil {
		return err
	}
	return u.send(evs)
}

func (u *uinput) Mouse(m layer.Mouse) error { return u.send(mouseEvents(m)) }

func (u *uinput) Close() error {
	_ = ioctl(u.f.Fd(), uiDevDestroy, 0)
	return u.f.Close()
}
