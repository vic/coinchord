package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"coinchord/internal/gesture"
	"coinchord/internal/layer"
)

type layAct uint8

const (
	layView layAct = iota
	layPlay
	layActivate
	layClear
)

type layMsg struct {
	act   layAct
	name  string
	chord gesture.Chord
}

type layState struct {
	shown  string // layer shown in the list
	top    int
	typist layer.Typist
	note   string
}

func (l *layState) on(m *Model, msg layMsg) {
	switch msg.act {
	case layView:
		l.shown, l.top = msg.name, 0
	case layPlay:
		l.apply(m, msg.chord)
	case layActivate:
		l.typist.Active = msg.name
	case layClear:
		active := l.typist.Active
		l.typist.Reset()
		l.typist.Active = active
	}
}

// apply types a chord into the sandbox, like live typing would.
func (l *layState) apply(m *Model, c gesture.Chord) {
	e, ok := m.prof.Layers.Lookup(l.typist.Active, c)
	if !ok {
		l.note = fmt.Sprintf("%s is not bound in %s. Bind it in Settings → Bindings.", c, l.typist.Active)
		return
	}
	l.typist.Apply(e)
	l.note = c.String() + " → " + e.Show()
	l.shown = l.typist.Active
}

func (l layState) view(m Model, w, h int) box {
	set := &m.prof.Layers
	var tabs []box
	for _, ly := range append([]layer.Layer{set.Global}, set.Layers...) {
		st := lipgloss.NewStyle().Padding(0, 1).Foreground(cDim)
		label := ly.Name
		if ly.Name == l.typist.Active {
			label = "● " + label
		}
		if ly.Name == l.shown {
			st = st.Foreground(lipgloss.Color("#000000")).Background(cAccent).Bold(true)
		}
		tabs = append(tabs, click(text(st.Render(label)), layMsg{act: layView, name: ly.Name}))
	}
	cur := set.Global
	if ly, ok := set.Find(l.shown); ok {
		cur = *ly
	}
	rows := []box{text(sDim.Render(fmt.Sprintf("%d bindings · click a row to play it in the sandbox", len(cur.Bind))))}
	chords := cur.Sorted(m.prof.Kit)
	listH := max(5, h-6)
	top := min(l.top, max(0, len(chords)-listH))
	for _, c := range chords[top:min(len(chords), top+listH)] {
		mark := " "
		if !layer.Reachable(m.prof.Kit, c) {
			mark = sBad.Render("✘")
		}
		left := lipgloss.NewStyle().Width(44).Render(mark + " " + chordChip(c))
		rows = append(rows, click(text(left+sBold.Render(cur.Bind[c].Show())), layMsg{act: layPlay, chord: c}))
	}
	if len(chords) > listH {
		rows = append(rows, text(sDim.Render(fmt.Sprintf("… %d-%d of %d, scroll for more", top+1, min(len(chords), top+listH), len(chords)))))
	}
	list := wrap(sPanel.Width(min(80, w/2+10)), vjoin(rows...))

	var switches []string
	for _, c := range set.Global.Sorted(m.prof.Kit) {
		switches = append(switches, chordChip(c)+sDim.Render(" "+set.Global.Bind[c].Show()))
	}
	var pressed []string
	for _, p := range l.typist.Pressed[max(0, len(l.typist.Pressed)-6):] {
		pressed = append(pressed, "⟨"+p.String()+"⟩")
	}
	sandbox := wrap(sPanel.Width(max(30, w-min(80, w/2+10)-3)), vjoin(
		text(sTitle.Render("Sandbox")+sDim.Render("  live chords type here · active layer ")+sOK.Render(l.typist.Active)),
		text(lipgloss.NewStyle().Width(max(20, w-min(80, w/2+10)-8)).Render(sBold.Render(strings.ReplaceAll(l.typist.Text(), "\t", "⇥"))+sOK.Render("▌"))),
		text(sDim.Render("shortcuts: ")+strings.Join(pressed, " ")),
		text(sDim.Render(l.note)),
		text(""),
		hjoin(1, button("Make "+l.shown+" active", layMsg{act: layActivate, name: l.shown}), button("Clear", layMsg{act: layClear})),
		text(""),
		text(sBold.Render("Layer switches (work from any layer)")),
		text(strings.Join(switches, "\n")),
		text(""),
		text(sDim.Render("Edit or add layers in Settings → Bindings.")),
	))
	return vjoin(hjoin(0, tabs...), hjoin(1, list, sandbox))
}
