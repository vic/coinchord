package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"coinchord/internal/gesture"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

type setPage uint8

const (
	pageKnobs setPage = iota
	pageBinds
	pageStories
)

type setMode uint8

const (
	modeIdle      setMode = iota
	modeRecord            // waiting for a chord
	modeExpansion         // typing the expansion of chord
	modeNewLayer          // typing a layer name
)

type setAct uint8

const (
	setPageAct setAct = iota
	setKnob
	setKnobsReset
	setLayerPick
	setRecord
	setEdit
	setDelete
	setNewLayer
	setSave
	setCancel
	setLayersReset
	setReload
	setAddWords
)

type setMsg struct {
	act   setAct
	i     int
	delta float64
	name  string
	chord gesture.Chord
}

type setState struct {
	page    setPage
	top     int
	layer   string
	mode    setMode
	chord   gesture.Chord
	input   textinput.Model
	err     string
	note    string
	confirm bool
}

func (s *setState) editing() bool { return s.mode != modeIdle }

func (s *setState) on(m *Model, msg setMsg) tea.Cmd {
	s.err = ""
	switch msg.act {
	case setPageAct:
		s.page, s.top = setPage(msg.i), 0
	case setKnob:
		k := gesture.Knobs[msg.i]
		*k.Of(&m.prof.Tuning) += msg.delta * k.Step
		m.prof.Tuning = m.prof.Tuning.Clamp()
		m.save()
	case setKnobsReset:
		d, t := gesture.DefaultTuning(), m.prof.Tuning
		d.Trigger, d.TapPeak, d.ScrapeFlat = t.Trigger, t.TapPeak, t.ScrapeFlat // keep calibrated values
		m.prof.Tuning = d
		m.save()
		s.note = "Knobs reset. Calibrated values kept."
	case setLayerPick:
		s.layer, s.top = msg.name, 0
	case setRecord:
		s.mode, s.note = modeRecord, ""
	case setEdit:
		return s.ask(modeExpansion, msg.chord, s.bound(m, msg.chord))
	case setDelete:
		if l := s.cur(m); l != nil {
			delete(l.Bind, msg.chord)
			m.save()
			s.note = "Deleted " + msg.chord.String()
		}
	case setNewLayer:
		return s.ask(modeNewLayer, gesture.Chord{}, "")
	case setSave:
		s.commit(m)
	case setCancel:
		s.mode = modeIdle
	case setLayersReset:
		if !s.confirm {
			s.confirm = true
			return nil
		}
		s.confirm = false
		set, err := layer.Defaults(m.prof.Kit, tutor.Words(m.opt.Stories))
		if err != nil {
			s.err = err.Error()
			return nil
		}
		m.prof.Layers = set
		m.save()
		s.note = "Layers reset to defaults."
	case setReload:
		user, err := tutor.Load(m.opt.StoriesDir)
		m.opt.Stories = append(slices.Clone(tutor.Builtin), user...)
		m.tut = tutState{}
		s.note = fmt.Sprintf("Loaded %d stories from %s.", len(user), m.opt.StoriesDir)
		if err != nil {
			s.err = err.Error()
		}
	case setAddWords:
		s.addWords(m)
	}
	return nil
}

func (s *setState) cur(m *Model) *layer.Layer {
	if s.layer == "global" {
		return &m.prof.Layers.Global
	}
	l, _ := m.prof.Layers.Find(s.layer)
	return l
}

func (s *setState) bound(m *Model, c gesture.Chord) string {
	if l := s.cur(m); l != nil {
		if e, ok := l.Bind[c]; ok {
			return e.Src()
		}
	}
	return ""
}

func (s *setState) ask(mode setMode, c gesture.Chord, value string) tea.Cmd {
	s.mode, s.chord = mode, c
	s.input = textinput.New()
	s.input.Placeholder = map[setMode]string{modeExpansion: "text, {enter}, {ctrl+alt+left}, {layer:kbd}, {undo}", modeNewLayer: "layer name"}[mode]
	s.input.SetWidth(60)
	s.input.SetValue(value)
	return s.input.Focus()
}

func (s *setState) onChord(m *Model, c gesture.Chord) {
	if s.mode != modeRecord {
		return
	}
	s.ask(modeExpansion, c, s.bound(m, c))
	if _, sw := m.prof.Layers.Global.Bind[c]; sw && s.layer != "global" {
		s.err = "Note: this chord is a layer switch. The switch wins over this binding."
	}
}

func (s *setState) commit(m *Model) {
	v := s.input.Value()
	switch s.mode {
	case modeExpansion:
		e, err := layer.Parse(v)
		if err != nil {
			s.err = err.Error()
			return
		}
		l := s.cur(m)
		if l == nil {
			s.err = "pick a layer first"
			return
		}
		l.Bind[s.chord] = e
		s.note = s.chord.String() + " → " + e.Show()
	case modeNewLayer:
		name := strings.TrimSpace(v)
		if _, dup := m.prof.Layers.Find(name); dup || name == "" || name == "global" || strings.ContainsAny(name, " {}:") {
			s.err = "Layer names must be new, not empty, without spaces, { } or :"
			return
		}
		c, ok := m.prof.Layers.Unused(m.prof.Kit)
		if !ok {
			s.err = "No free chord left for a layer switch."
			return
		}
		sw, err := layer.Parse("{layer:" + name + "}")
		if err != nil {
			s.err = err.Error()
			return
		}
		m.prof.Layers.Layers = append(m.prof.Layers.Layers, layer.Layer{Name: name, Bind: map[gesture.Chord]layer.Expansion{}})
		m.prof.Layers.Global.Bind[c] = sw
		s.layer = name
		s.note = fmt.Sprintf("Layer %s added. Switch to it with %s.", name, c)
	}
	s.mode = modeIdle
	m.save()
}

// addWords binds every story word missing from the english layer.
func (s *setState) addWords(m *Model) {
	en, ok := m.prof.Layers.Find("english")
	if !ok {
		s.err = "no english layer"
		return
	}
	have := map[string]bool{}
	for _, e := range en.Bind {
		have[strings.ToLower(e.Src())] = true
	}
	added := 0
	for _, w := range missingWords(m) {
		c, ok := m.prof.Layers.Unused(m.prof.Kit)
		e, err := layer.Parse(w + " ")
		if !ok || err != nil || have[w+" "] {
			continue
		}
		en.Bind[c] = e
		have[w+" "] = true
		added++
	}
	m.save()
	s.note = fmt.Sprintf("Added %d words to english.", added)
}

func missingWords(m *Model) []string {
	en, ok := m.prof.Layers.Find("english")
	if !ok {
		return nil
	}
	have := map[string]bool{}
	for _, e := range en.Bind {
		have[strings.ToLower(strings.TrimSpace(e.Src()))] = true
	}
	var out []string
	for _, w := range tutor.Words(m.opt.Stories) {
		w = strings.ToLower(strings.Trim(w, ".,!?;:\"()"))
		if w != "" && !have[w] && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

func (s *setState) onKey(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc":
			s.mode = modeIdle
			return *m, nil
		case "enter":
			if s.mode != modeRecord {
				s.commit(m)
			}
			return *m, nil
		case "ctrl+c":
			return *m, tea.Quit
		}
	}
	if s.mode == modeRecord {
		return *m, nil
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return *m, cmd
}

func (s setState) view(m Model, w, h int) box {
	var pages []box
	for i, n := range []string{"Knobs", "Bindings", "Stories"} {
		st := lipgloss.NewStyle().Padding(0, 1).Foreground(cDim)
		if setPage(i) == s.page {
			st = st.Foreground(lipgloss.Color("#000000")).Background(cAccent).Bold(true)
		}
		pages = append(pages, click(text(st.Render(n)), setMsg{act: setPageAct, i: i}))
	}
	var body box
	switch s.page {
	case pageKnobs:
		body = s.viewKnobs(m)
	case pageBinds:
		body = s.viewBinds(m, w, h)
	default:
		body = s.viewStories(m)
	}
	var notes []box
	if s.err != "" {
		notes = append(notes, text(sBad.Render(s.err)))
	}
	if s.note != "" {
		notes = append(notes, text(sOK.Render(s.note)))
	}
	return vjoin(append([]box{hjoin(1, pages...)}, append(notes, wrap(sPanel.Width(w-2), body))...)...)
}

func (s setState) viewKnobs(m Model) box {
	rows := []box{text(sDim.Render("Small steps help. Each change applies at once and is saved."))}
	for i, k := range gesture.Knobs {
		t := m.prof.Tuning
		v := *k.Of(&t)
		val := sBold.Render(fmt.Sprintf("%9.3f", v))
		if k.Step >= 1 {
			val = sBold.Render(fmt.Sprintf("%9.0f", v))
		}
		rows = append(rows, hjoin(1,
			text(lipgloss.NewStyle().Width(22).Render(k.Name)),
			click(text(sChip.Render(" − ")), setMsg{act: setKnob, i: i, delta: -1}),
			text(val),
			click(text(sChip.Render(" + ")), setMsg{act: setKnob, i: i, delta: 1}),
			text(bar(v-k.Min, k.Max-k.Min, 16, cAccent, -1)+" "+sDim.Render(k.Help)),
		))
	}
	rows = append(rows, button("Reset knobs (keeps calibration)", setMsg{act: setKnobsReset}))
	return vjoin(rows...)
}

func (s setState) viewBinds(m Model, w, h int) box {
	set := &m.prof.Layers
	var tabs []box
	for _, l := range append([]layer.Layer{set.Global}, set.Layers...) {
		st := lipgloss.NewStyle().Padding(0, 1).Foreground(cDim)
		if l.Name == s.layer {
			st = st.Foreground(lipgloss.Color("#000000")).Background(cAccent).Bold(true)
		}
		tabs = append(tabs, click(text(st.Render(l.Name)), setMsg{act: setLayerPick, name: l.Name}))
	}
	tabs = append(tabs, link("+ new layer", setMsg{act: setNewLayer}))
	var editor box
	switch s.mode {
	case modeRecord:
		editor = vjoin(text(sWarn.Bold(true).Render("● Recording… perform the chord now (one or two gestures).")),
			button("Cancel", setMsg{act: setCancel}))
	case modeExpansion, modeNewLayer:
		title := "New layer name"
		if s.mode == modeExpansion {
			title = "Expansion for " + chordChip(s.chord) + " in " + s.layer
		}
		editor = vjoin(text(sBold.Render(title)), text(s.input.View()),
			text(sDim.Render("Tokens: {enter} {bs} {tab} {space} {esc} {left} … {ctrl+alt+left} {super+q} {layer:NAME} {undo}  ·  {{ types {")),
			hjoin(1, hotButton("Save (enter)", setMsg{act: setSave}), button("Cancel (esc)", setMsg{act: setCancel})))
	default:
		editor = hjoin(1, hotButton("● Record a new binding", setMsg{act: setRecord}), button(map[bool]string{false: "Reset all layers", true: "Click again to reset ALL layers"}[s.confirm], setMsg{act: setLayersReset}))
	}
	l := s.cur(&m)
	rows := []box{}
	if l != nil {
		chords := l.Sorted(m.prof.Kit)
		listH := max(4, h-12)
		top := min(s.top, max(0, len(chords)-listH))
		for _, c := range chords[top:min(len(chords), top+listH)] {
			rows = append(rows, hjoin(1,
				link("edit", setMsg{act: setEdit, chord: c}), link("del", setMsg{act: setDelete, chord: c}),
				text(lipgloss.NewStyle().Width(44).Render(chordChip(c))+sBold.Render(l.Bind[c].Show()))))
		}
		rows = append(rows, text(sDim.Render(fmt.Sprintf("%d bindings, scroll for more", len(chords)))))
	}
	return vjoin(hjoin(0, tabs...), text(""), editor, text(""), vjoin(rows...))
}

func (s setState) viewStories(m Model) box {
	var rows []string
	for _, st := range m.opt.Stories {
		rows = append(rows, fmt.Sprintf("• %s %s", st.Title, sDim.Render(fmt.Sprintf("(%d sentences)", len(st.Sentences())))))
	}
	miss := missingWords(&m)
	return vjoin(
		text(sTitle.Render("Practice texts")),
		text("Put any .txt file in "+sBold.Render(m.opt.StoriesDir)+" and click Reload. Or start with  "+sBold.Render("coinchord -text file.txt")),
		text(strings.Join(rows, "\n")),
		text(""),
		text(fmt.Sprintf("%d words of your texts have no english chord yet. The tutor spells them with the alpha layer.", len(miss))),
		text(sDim.Render(trunc(strings.Join(miss, " "), 200))),
		text(""),
		hjoin(1, button("Reload", setMsg{act: setReload}), button("Give every word its own chord", setMsg{act: setAddWords})),
	)
}
