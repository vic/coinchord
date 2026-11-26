// Package app is the CoinChord engine. One goroutine (Run) owns all state;
// audio analysis, host output, system calls and browser clients run on
// their own goroutines and talk to it through channels.
package app

import (
	"context"
	"encoding/json"
	"io"
	"runtime"
	"sync/atomic"
	"time"

	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/gesture"
	"coinchord/internal/inject"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

// Options configure the engine.
type Options struct {
	ProfilePath string
	StoriesDir  string
	Stories     []tutor.Story
	OpenDevice  func() (inject.Device, error)
	Output      bool      // start with host output on
	Notice      string    // shown once at start
	Labels      io.Writer // calibration labels for -eval; nil = not recording
}

const (
	eventBuffer  = 256
	clientBuffer = 128
	tickEvery    = 50 * time.Millisecond
	levelEvery   = 60 // ticks between system mic volume reads (~3 s)
	flashFor     = 900 * time.Millisecond
)

// App is the engine. Create it with New, start it with Run.
type App struct {
	opt    Options
	setup  atomic.Pointer[audio.Setup] // read by the analysis goroutine
	events chan any
	cmds   chan Cmd
	joins  chan *client
	leaves chan *client
	done   chan struct{} // closed when Run returns

	// owned by Run
	clients map[*client]bool
	prof    config.Profile
	mon     audio.Monitor
	level   *audio.Level
	micErr  string
	notice  string
	out     *output
	cal     cal
	tut     tut
	sand    layer.Typist // the live typist: active layer, sandbox text
	view    string       // layer shown in the Layers page
	rec     recording
	heard   map[gesture.Gesture]int
	last    *audio.Stroke
	chord   *audio.Chord
	typed   string // what the last chord did
	ticks   int
	now     time.Time
	dirty   bool
	page    page // what the browser shows
}

type recording struct {
	listening bool
	chord     *gesture.Chord
}

type client struct{ ch chan []byte }

// New builds the engine from a loaded profile.
func New(p config.Profile, opt Options) *App {
	if opt.OpenDevice == nil {
		opt.OpenDevice = inject.Open
	}
	a := &App{
		opt: opt, prof: p, notice: opt.Notice, now: time.Now(),
		events: make(chan any, eventBuffer), cmds: make(chan Cmd, eventBuffer),
		joins: make(chan *client), leaves: make(chan *client), done: make(chan struct{}),
		clients: map[*client]bool{}, heard: map[gesture.Gesture]int{}, view: "english",
	}
	a.sand.Active = "english"
	a.tut.drill = -1
	a.publish()
	return a
}

// Setup is read by the audio analysis goroutine.
func (a *App) Setup() *atomic.Pointer[audio.Setup] { return &a.setup }

// Emit receives audio events. Frames are dropped when the engine is busy,
// so analysis never waits; strokes and chords always arrive.
func (a *App) Emit(e any) {
	if _, frame := e.(audio.Frame); frame {
		select {
		case a.events <- e:
		default:
		}
		return
	}
	select {
	case a.events <- e:
	case <-a.done:
	}
}

// Do queues a command from a client.
func (a *App) Do(c Cmd) {
	select {
	case a.cmds <- c:
	case <-a.done:
	}
}

// Join registers a browser client; it receives the full state first.
func (a *App) Join() (<-chan []byte, func()) {
	c := &client{make(chan []byte, clientBuffer)}
	select {
	case a.joins <- c:
	case <-a.done:
		close(c.ch)
	}
	return c.ch, func() {
		select {
		case a.leaves <- c:
		case <-a.done:
		}
	}
}

func (a *App) publish() { a.setup.Store(&audio.Setup{Kit: a.prof.Kit, Tuning: a.prof.Tuning}) }

func (a *App) save() {
	a.publish()
	a.dirty = true
	if err := config.Save(a.opt.ProfilePath, a.prof); err != nil {
		a.notice = "Could not save your profile: " + err.Error()
	}
}

// Run is the engine loop. It returns when ctx ends.
func (a *App) Run(ctx context.Context) {
	if a.opt.Output {
		go func() { a.Emit(openOutput(a.opt.OpenDevice)) }()
	}
	go a.readLevel()
	t := time.NewTicker(tickEvery)
	defer t.Stop()
	defer close(a.done)
	for {
		select {
		case <-ctx.Done():
			if a.out != nil {
				close(a.out.q)
			}
			return
		case c := <-a.joins:
			a.clients[c] = true
			a.sendTo(c, a.stateJSON(), true)
		case c := <-a.leaves:
			if a.clients[c] { // a slow client may already be gone
				delete(a.clients, c)
				close(c.ch)
			}
		case e := <-a.events:
			a.onEvent(e)
		case c := <-a.cmds:
			a.onCmd(c)
		case now := <-t.C:
			a.now = now
			a.ticks++
			a.tut.tick(a)
			if a.ticks%levelEvery == 0 {
				go a.readLevel()
			}
			if a.dirty {
				a.dirty = false
				a.broadcast(a.stateJSON(), true)
			}
		}
	}
}

// broadcast sends b to every client. Frames are dropped for slow clients;
// a client too slow for state updates is disconnected and reconnects.
func (a *App) broadcast(b []byte, must bool) {
	for c := range a.clients {
		a.sendTo(c, b, must)
	}
}

func (a *App) sendTo(c *client, b []byte, must bool) {
	select {
	case c.ch <- b:
	default:
		if must {
			delete(a.clients, c)
			close(c.ch)
		}
	}
}

type (
	levelMsg  struct{ level *audio.Level }
	noticeMsg string
)

func (a *App) readLevel() {
	l, err := audio.ReadLevel()
	if err != nil {
		a.Emit(levelMsg{nil})
		return
	}
	a.Emit(levelMsg{&l})
}

func (a *App) onEvent(e any) {
	switch e := e.(type) {
	case audio.Frame:
		a.mon.Add(e.Frame)
		a.cal.onFrame(a, e)
		a.broadcast(frameJSON(a, e), false)
	case audio.Stroke:
		a.onStroke(e)
	case audio.Chord:
		a.onChord(e)
	case audio.Stopped:
		a.micErr = "the microphone stopped: " + e.Err.Error()
		a.dirty = true
	case levelMsg:
		a.level = e.level
		a.dirty = true
	case noticeMsg:
		a.notice = string(e)
		a.dirty = true
	case outReady:
		a.outReady(e)
	case outErr:
		a.notice = "Output: " + e.err.Error()
		a.dirty = true
	}
}

func (a *App) onStroke(s audio.Stroke) {
	a.last = &s
	a.heard[s.Gesture]++
	a.cal.onStroke(a, s)
	a.broadcast(strokeJSON(a, s), false)
	a.dirty = true
}

func (a *App) onChord(c audio.Chord) {
	a.chord = &c
	a.dirty = true
	switch {
	case a.rec.listening:
		a.rec.listening, a.rec.chord = false, &c.Chord
	case a.page == pageCalibrate:
		// calibrating: chords do nothing
	case a.page == pageTutor && a.tut.running():
		a.tut.onChord(a, c.Chord)
	default:
		a.apply(c.Chord)
	}
}

// apply runs a chord in the active layer: sandbox text and host output.
func (a *App) apply(c gesture.Chord) {
	e, ok := a.prof.Layers.Lookup(a.sand.Active, c)
	if !ok {
		a.typed = "nothing (" + c.String() + " is free in " + a.sand.Active + ")"
		return
	}
	a.sand.Apply(e)
	a.typed = e.Show()
	a.view = a.sand.Active
	a.emit(e)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"t":"error"}`) // our own types always marshal; tests cover it
	}
	return b
}

var goos = runtime.GOOS
