// Package inject sends chord actions to the host system as real keyboard
// and mouse input: a virtual device on Linux, SendInput on Windows and
// CoreGraphics events on macOS.
package inject

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"

	"coinchord/internal/layer"
)

// Device is one system backend.
type Device interface {
	Text(s string) error // printable text, no control characters
	Combo(c layer.Combo) error
	Mouse(m layer.Mouse) error
	Close() error
}

// Host applies expansions to a Device.
type Host struct {
	dev  Device
	goos string
	last int // runes typed by the previous expansion, for {undo}
}

// New wraps a device for system goos (a GOOS value).
func New(d Device, goos string) *Host { return &Host{dev: d, goos: goos} }

// Close releases the device.
func (h *Host) Close() error { return h.dev.Close() }

var (
	enter = layer.Combo{Key: "enter"}
	tab   = layer.Combo{Key: "tab"}
	bs    = layer.Combo{Key: "bs"}
)

// Run performs every action of e on the host. Layer switches are not host
// actions; the caller handles them.
func (h *Host) Run(e layer.Expansion) error {
	typed := 0
	for _, a := range e.Actions() {
		var err error
		switch a := a.(type) {
		case layer.Text:
			typed += utf8.RuneCountInString(string(a))
			err = h.text(string(a))
		case layer.Combo:
			err = h.dev.Combo(a)
		case layer.Mouse:
			err = h.dev.Mouse(a)
		case layer.Command:
			var c layer.Combo
			if c, err = a.Shortcut(h.goos); err == nil {
				err = h.dev.Combo(c)
			}
		case layer.Undo:
			for range h.last {
				if err = h.dev.Combo(bs); err != nil {
					break
				}
			}
			h.last = 0
		}
		if err != nil {
			return err
		}
	}
	if typed > 0 {
		h.last = typed
	}
	return nil
}

// text types s, sending newlines and tabs as keys.
func (h *Host) text(s string) error {
	for s != "" {
		i := strings.IndexAny(s, "\n\t")
		if i < 0 {
			return h.dev.Text(s)
		}
		if i > 0 {
			if err := h.dev.Text(s[:i]); err != nil {
				return err
			}
		}
		k := map[byte]layer.Combo{'\n': enter, '\t': tab}[s[i]]
		if err := h.dev.Combo(k); err != nil {
			return err
		}
		s = s[i+1:]
	}
	return nil
}

// errUnsupported is returned on systems without a backend.
var errUnsupported = fmt.Errorf("host input is not supported on %s", runtime.GOOS)
