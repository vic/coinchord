package app

import (
	"coinchord/internal/inject"
	"coinchord/internal/layer"
)

// output sends expansions to the host on its own goroutine, in order.
type output struct {
	q    chan layer.Expansion
	errs chan error
}

type (
	outReady struct {
		out *output
		err error
	}
	outErr struct{ err error }
)

const queueSize = 64

// openOutput opens the device and starts the output goroutine. Errors of
// later actions come back through errs.
func openOutput(open func() (inject.Device, error)) outReady {
	d, err := open()
	if err != nil {
		return outReady{nil, err}
	}
	return outReady{&output{q: make(chan layer.Expansion, queueSize)}, nil}.start(inject.New(d, goos))
}

func (r outReady) start(h *inject.Host) outReady {
	errs := make(chan error, queueSize)
	r.out.errs = errs
	go func() {
		defer close(errs) // ends the error forwarder
		defer h.Close()
		for e := range r.out.q {
			if err := h.Run(e); err != nil {
				select {
				case errs <- err:
				default:
				}
			}
		}
	}()
	return r
}

func (a *App) emit(e layer.Expansion) {
	if a.out == nil {
		return
	}
	select {
	case a.out.q <- e:
	default:
		a.notice = "Output is busy; one chord was skipped."
	}
}

func (a *App) setOutput(on bool) {
	switch {
	case on && a.out == nil:
		go func() { a.Emit(openOutput(a.opt.OpenDevice)) }()
	case !on && a.out != nil:
		close(a.out.q)
		a.out = nil
		a.notice = "Output is off. Chords only show here."
	}
	a.dirty = true
}

func (a *App) outReady(r outReady) {
	a.dirty = true
	if r.err != nil {
		a.notice = "Output could not start: " + r.err.Error()
		return
	}
	if a.out != nil { // turned on twice
		close(r.out.q)
		return
	}
	a.out = r.out
	a.notice = "Output is on: chords now type, click and switch windows on your computer."
	go func(errs <-chan error) {
		for err := range errs {
			a.Emit(outErr{err})
		}
	}(r.out.errs)
}
