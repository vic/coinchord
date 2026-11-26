//go:build linux

package inject

import (
	"bytes"
	"encoding/binary"
	"testing"

	"coinchord/internal/layer"
)

func codes(evs []event) []uint16 {
	var out []uint16
	for _, e := range evs {
		if e.typ == evKey && e.value == 1 {
			out = append(out, e.code)
		}
	}
	return out
}

func TestLinuxEvents(t *testing.T) {
	// "Hi!" = shift+h, i, shift+1
	got := codes(textEvents("Hi!"))
	want := []uint16{keyLeftShift, 35, 23, keyLeftShift, 2}
	if !equal(got, want) {
		t.Fatalf("text: %v want %v", got, want)
	}
	c, _ := layer.Parse("{ctrl+alt+left}")
	evs, err := comboEvents(c.Actions()[0].(layer.Combo))
	if err != nil || !equal(codes(evs), []uint16{keyLeftCtrl, keyLeftAlt, 105}) {
		t.Fatalf("combo: %v %v", codes(evs), err)
	}
	// releases mirror presses, in reverse
	if last := evs[len(evs)-2]; last != (event{evKey, keyLeftCtrl, 0}) {
		t.Fatalf("last release %v", last)
	}
	// ñ goes through ctrl+shift+u f1 space
	if got := codes(textEvents("ñ")); !equal(got, []uint16{keyLeftCtrl, keyLeftShift, 22, 33, 2, 57}) {
		t.Fatalf("unicode: %v", got)
	}
	sc := mouseEvents(layer.Mouse{Kind: layer.Scroll, DY: 3})
	if sc[0] != (event{evRel, relWheel, -3}) {
		t.Fatalf("scroll down must be a negative wheel: %v", sc[0])
	}
	var b bytes.Buffer
	if err := encode(&b, []event{{evKey, 30, 1}}); err != nil || b.Len() != 24 {
		t.Fatalf("encode %d bytes", b.Len())
	}
	if binary.NativeEndian.Uint16(b.Bytes()[18:]) != 30 || binary.NativeEndian.Uint32(b.Bytes()[20:]) != 1 {
		t.Fatalf("layout %x", b.Bytes())
	}
}

func equal(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
