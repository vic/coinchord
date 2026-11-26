//go:build windows

package inject

import (
	"fmt"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"coinchord/internal/layer"
)

var (
	user32    = syscall.NewLazyDLL("user32.dll")
	sendInput = user32.NewProc("SendInput")
	vkScan    = user32.NewProc("VkKeyScanW")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyExtended = 0x0001
	keyUp       = 0x0002
	keyUnicode  = 0x0004

	mouseMove       = 0x0001
	mouseLeftDown   = 0x0002
	mouseLeftUp     = 0x0004
	mouseRightDown  = 0x0008
	mouseRightUp    = 0x0010
	mouseMiddleDown = 0x0020
	mouseMiddleUp   = 0x0040
	mouseWheel      = 0x0800
	mouseHWheel     = 0x1000
	wheelDelta      = 120

	vkShift = 0x10
	vkCtrl  = 0x11
	vkAlt   = 0x12
	vkWin   = 0x5B
)

// keybdInput and mouseInput mirror KEYBDINPUT and MOUSEINPUT; the INPUT
// union is as large as MOUSEINPUT, so keyboard entries carry padding.
type keybdInput struct {
	vk, scan    uint16
	flags, time uint32
	extra       uintptr
	_           [8]byte
}

type mouseInput struct {
	dx, dy            int32
	data, flags, time uint32
	extra             uintptr
}

type keyInput struct {
	typ uint32
	ki  keybdInput
}

type ptrInput struct {
	typ uint32
	mi  mouseInput
}

var namedVK = map[string]uint16{
	"enter": 0x0D, "bs": 0x08, "tab": 0x09, "space": 0x20, "esc": 0x1B, "del": 0x2E,
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28, "home": 0x24, "end": 0x23, "pgup": 0x21, "pgdn": 0x22,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74, "f6": 0x75, "f7": 0x76, "f8": 0x77,
	"f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
}

// extended keys need KEYEVENTF_EXTENDEDKEY or Windows reads them as numpad keys
var extendedVK = map[uint16]bool{0x2E: true, 0x25: true, 0x26: true, 0x27: true, 0x28: true, 0x24: true, 0x23: true, 0x21: true, 0x22: true, vkWin: true}

type win struct{}

// Open returns the SendInput backend.
func Open() (Device, error) {
	if err := sendInput.Find(); err != nil {
		return nil, err
	}
	return win{}, nil
}

func send[T keyInput | ptrInput](in []T) error {
	if len(in) == 0 {
		return nil
	}
	n, _, err := sendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
	if int(n) != len(in) {
		return fmt.Errorf("SendInput sent %d of %d events: %v (input blocked by a higher-privilege window?)", n, len(in), err)
	}
	return nil
}

func key(vk uint16, up bool) keyInput {
	var f uint32
	if up {
		f |= keyUp
	}
	if extendedVK[vk] {
		f |= keyExtended
	}
	return keyInput{typ: inputKeyboard, ki: keybdInput{vk: vk, flags: f}}
}

func (win) Text(s string) error {
	var in []keyInput
	for _, u := range utf16.Encode([]rune(s)) {
		in = append(in,
			keyInput{typ: inputKeyboard, ki: keybdInput{scan: u, flags: keyUnicode}},
			keyInput{typ: inputKeyboard, ki: keybdInput{scan: u, flags: keyUnicode | keyUp}})
	}
	return send(in)
}

func (win) Combo(c layer.Combo) error {
	var mods []uint16
	for _, m := range []struct {
		m  layer.Mod
		vk uint16
	}{{layer.Ctrl, vkCtrl}, {layer.Alt, vkAlt}, {layer.Shift, vkShift}, {layer.Super, vkWin}} {
		if c.Mods&m.m != 0 {
			mods = append(mods, m.vk)
		}
	}
	vk, ok := namedVK[c.Key]
	if !ok {
		r, _, _ := vkScan.Call(uintptr([]rune(c.Key)[0]))
		if int16(r) == -1 {
			return fmt.Errorf("no key for %q on this keyboard layout", c.Key)
		}
		vk = uint16(r & 0xFF)
		if r&0x100 != 0 {
			mods = append(mods, vkShift)
		}
	}
	var in []keyInput
	for _, m := range mods {
		in = append(in, key(m, false))
	}
	in = append(in, key(vk, false), key(vk, true))
	for i := len(mods) - 1; i >= 0; i-- {
		in = append(in, key(mods[i], true))
	}
	return send(in)
}

var buttonFlags = map[layer.Button][2]uint32{
	layer.LeftButton:   {mouseLeftDown, mouseLeftUp},
	layer.RightButton:  {mouseRightDown, mouseRightUp},
	layer.MiddleButton: {mouseMiddleDown, mouseMiddleUp},
}

func ptr(dx, dy int32, data, flags uint32) ptrInput {
	return ptrInput{typ: inputMouse, mi: mouseInput{dx: dx, dy: dy, data: data, flags: flags}}
}

func (win) Mouse(m layer.Mouse) error {
	b := buttonFlags[m.Button]
	switch m.Kind {
	case layer.Move:
		return send([]ptrInput{ptr(int32(m.DX), int32(m.DY), 0, mouseMove)})
	case layer.Scroll: // wheel up is positive; our DY > 0 means down
		return send([]ptrInput{ptr(0, 0, uint32(int32(-m.DY*wheelDelta)), mouseWheel), ptr(0, 0, uint32(int32(m.DX*wheelDelta)), mouseHWheel)})
	case layer.Down:
		return send([]ptrInput{ptr(0, 0, 0, b[0])})
	case layer.Up:
		return send([]ptrInput{ptr(0, 0, 0, b[1])})
	case layer.Double:
		if err := send([]ptrInput{ptr(0, 0, 0, b[0]), ptr(0, 0, 0, b[1])}); err != nil {
			return err
		}
		time.Sleep(30 * time.Millisecond)
	}
	return send([]ptrInput{ptr(0, 0, 0, b[0]), ptr(0, 0, 0, b[1])})
}

func (win) Close() error { return nil }
