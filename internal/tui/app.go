// Package tui is the CoinChord terminal interface. Every control is clickable.
package tui

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"coinchord/internal/audio"
	"coinchord/internal/config"
	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

type tab uint8

const (
	tabOverview tab = iota
	tabCalibrate
	tabGestures
	tabTutor
	tabLayers
	tabSettings
	tabCount
)

var tabNames = [tabCount]string{"Overview", "Calibrate", "Gestures", "Tutor", "Layers", "Settings"}

type (
	tabMsg  tab
	tickMsg time.Time
	quitMsg struct{}
)

// Options configure the app.
type Options struct {
	ProfilePath string
	StoriesDir  string
	Stories     []tutor.Story // built-in plus user stories
	Setup       *atomic.Pointer[audio.Setup]
	Notice      string // shown once at start (for example a profile load problem)
}

// Model is the root bubbletea model.
type Model struct {
	opt    Options
	prof   config.Profile
	w, h   int
	tab    tab
	notice string
	now    time.Time

	mic     string // "" = listening; else the problem
	frame   dsp.Frame
	flashes map[gesture.Striker]flash
	strokes []audio.Stroke // newest last
	chord   *audio.Chord
	heard   string // what the last chord typed in the active layer

	cal calState
	gal galState
	tut tutState
	lay layState
	set setState
}

// New builds the model from a loaded profile.
func New(p config.Profile, opt Options) Model {
	m := Model{opt: opt, prof: p, notice: opt.Notice, now: time.Now(), flashes: map[gesture.Striker]flash{}}
	m.lay.typist.Active = "english"
	m.lay.shown = "english"
	m.set.layer = "english"
	if !p.Calibrated {
		m.tab = tabOverview
	}
	m.publish()
	return m
}

// publish hands the current kit and tuning to the audio listener.
func (m *Model) publish() {
	m.opt.Setup.Store(&audio.Setup{Kit: m.prof.Kit, Tuning: m.prof.Tuning})
}

// save stores the profile and republishes it.
func (m *Model) save() {
	m.publish()
	if err := config.Save(m.opt.ProfilePath, m.prof); err != nil {
		m.notice = "Could not save profile: " + err.Error()
	}
}

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Init() tea.Cmd { return tick() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.now = time.Time(msg)
		return m, tea.Batch(tick(), m.onTick())
	case quitMsg:
		return m, tea.Quit
	case tabMsg:
		m.tab = tab(msg)
		return m, nil
	case tea.KeyPressMsg:
		return m.onKey(msg)
	case tea.MouseWheelMsg:
		d := 3
		if msg.Button == tea.MouseWheelUp {
			d = -3
		}
		m.scroll(d)
		return m, nil
	case audio.Frame:
		m.frame = msg.Frame
		m.cal.onFrame(&m, msg)
		return m, nil
	case audio.Stroke:
		m.onStroke(msg)
		return m, nil
	case audio.Chord:
		m.onChord(msg)
		return m, nil
	case audio.Stopped:
		m.mic = "Input stopped: " + msg.Err.Error()
		return m, nil
	}
	return m.route(msg)
}

func (m *Model) onStroke(s audio.Stroke) {
	for _, st := range s.Gesture.Src.Strikers() {
		m.flashes[st] = flash{s.Gesture.Shape, m.now}
	}
	m.strokes = append(m.strokes, s)
	if len(m.strokes) > 8 {
		m.strokes = m.strokes[1:]
	}
	m.gal.onStroke(s, m.now)
	m.cal.onStroke(m, s)
}

func (m *Model) onChord(c audio.Chord) {
	m.chord = &c
	m.heard = ""
	if e, ok := m.prof.Layers.Lookup(m.lay.typist.Active, c.Chord); ok {
		m.heard = e.Show()
	}
	switch m.tab {
	case tabTutor:
		m.tut.onChord(m, c.Chord)
	case tabLayers:
		m.lay.apply(m, c.Chord)
	case tabSettings:
		m.set.onChord(m, c.Chord)
	}
}

func (m *Model) onTick() tea.Cmd {
	m.tut.onTick(m)
	return nil
}

func (m Model) onKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.tab == tabSettings && m.set.editing() {
		return m.set.onKey(&m, k)
	}
	switch k.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		m.tab = (m.tab + 1) % tabCount
	case "shift+tab":
		m.tab = (m.tab + tabCount - 1) % tabCount
	case "1", "2", "3", "4", "5", "6":
		m.tab = tab(k.String()[0] - '1')
	case "up", "k":
		m.scroll(-1)
	case "down", "j":
		m.scroll(1)
	case "esc":
		m.notice = ""
	}
	return m, nil
}

func (m *Model) scroll(d int) {
	switch m.tab {
	case tabGestures:
		m.gal.top = max(0, m.gal.top+d)
	case tabLayers:
		m.lay.top = max(0, m.lay.top+d)
	case tabSettings:
		m.set.top = max(0, m.set.top+d)
	case tabTutor:
		m.tut.top = max(0, m.tut.top+d)
	}
}

// route sends section messages to their section.
func (m Model) route(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case noticeOff:
		m.notice = ""
	case pickKit:
		m.pickKit(int(msg))
	case calMsg:
		m.cal.on(&m, msg)
	case galPick:
		m.gal.sel = &msg.g
	case tutMsg:
		m.tut.on(&m, msg)
	case layMsg:
		m.lay.on(&m, msg)
	case setMsg:
		cmd = m.set.on(&m, msg)
	default:
		if m.tab == tabSettings && m.set.editing() {
			var mm tea.Model
			mm, cmd = m.set.onKey(&m, msg)
			return mm, cmd
		}
	}
	return m, cmd
}

type noticeOff struct{}

// screen renders the whole screen with its click zones.
func (m Model) screen() box {
	w := max(m.w, 80)
	head := m.header()
	live := m.liveStrip(w)
	used := lipgloss.Height(head.s) + lipgloss.Height(live.s) + 2
	body := m.body(w, max(10, m.h-used))
	help := text(sDim.Render(" mouse: click anything · wheel scrolls · keys: tab/1-6 sections · q quits"))
	all := vjoin(head, live, body, help)
	if m.h > 0 {
		lines := strings.Split(all.s, "\n")
		if len(lines) > m.h {
			all.s = strings.Join(lines[:m.h], "\n")
		}
	}
	return all
}

func (m Model) View() tea.View {
	all := m.screen()
	v := tea.NewView(all.s)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "CoinChord"
	v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
		c, ok := msg.(tea.MouseClickMsg)
		if !ok || c.Button != tea.MouseLeft {
			return nil
		}
		hit, ok := all.hit(c.X, c.Y)
		if !ok {
			return nil
		}
		return func() tea.Msg { return hit }
	}
	return v
}

func (m Model) header() box {
	tabs := []box{text(sTitle.Render(" ◎ CoinChord "))}
	for i, n := range tabNames {
		st := lipgloss.NewStyle().Padding(0, 1).Foreground(cDim)
		if tab(i) == m.tab {
			st = st.Foreground(lipgloss.Color("#000000")).Background(cAccent).Bold(true)
		}
		tabs = append(tabs, click(text(st.Render(fmt.Sprintf("%d %s", i+1, n))), tabMsg(i)))
	}
	status := sDim.Render(fmt.Sprintf("  kit: %s · layer: %s", m.prof.Preset, m.lay.typist.Active))
	if !m.prof.Calibrated {
		status += sWarn.Render("  · not calibrated yet")
	}
	rows := []box{hjoin(0, append(tabs, text(status))...)}
	if m.notice != "" {
		rows = append(rows, hjoin(1, text(sWarn.Render(" ⚠ "+m.notice)), link("[dismiss]", noticeOff{})))
	}
	return vjoin(rows...)
}

// liveStrip is the always-visible detection display.
func (m Model) liveStrip(w int) box {
	mic := sOK.Render("● listening")
	if m.mic != "" {
		mic = sBad.Render("● " + m.mic)
	}
	meter := vjoin(
		text(mic),
		text(sDim.Render("loudness ")+level(m.frame.RMS, m.prof.Tuning.Trigger)),
		text(spectrum(m.frame, m.prof.Kit, m.prof.Tuning.Trigger, 2)),
	)
	var last []string
	for i := len(m.strokes) - 1; i >= 0 && len(last) < 3; i-- {
		last = append(last, chip(m.strokes[i].Gesture))
	}
	lastChord := sDim.Render("perform a gesture…")
	if m.chord != nil {
		lastChord = chordChip(m.chord.Chord)
		if m.heard != "" {
			lastChord += sDim.Render("  types ") + sBold.Render(m.heard)
		}
	}
	info := vjoin(
		text(sDim.Render("strokes  ")+strings.Join(last, sDim.Render(" · "))),
		text(sDim.Render("chord    ")+lastChord),
	)
	return wrap(sPanel.Width(w-2), hjoin(3, coins(m.prof.Kit, m.flashes, m.now), meter, info))
}

func (m Model) body(w, h int) box {
	switch m.tab {
	case tabOverview:
		return m.overview(w)
	case tabCalibrate:
		return m.cal.view(m, w)
	case tabGestures:
		return m.gal.view(m, w, h)
	case tabTutor:
		return m.tut.view(m, w, h)
	case tabLayers:
		return m.lay.view(m, w, h)
	}
	return m.set.view(m, w, h)
}

// lookupTyped explains what a chord types in a layer.
func lookupTyped(s *layer.Set, active string, c gesture.Chord) string {
	if e, ok := s.Lookup(active, c); ok {
		return e.Show()
	}
	return "nothing in " + active
}
