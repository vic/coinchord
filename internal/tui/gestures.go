package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"coinchord/internal/audio"
	"coinchord/internal/gesture"
)

type galPick struct{ g gesture.Gesture }

type galState struct {
	top    int
	sel    *gesture.Gesture
	counts map[gesture.Gesture]int
	seen   map[gesture.Gesture]time.Time
	last   *audio.Stroke
}

func (g *galState) onStroke(s audio.Stroke, now time.Time) {
	if g.counts == nil {
		g.counts, g.seen = map[gesture.Gesture]int{}, map[gesture.Gesture]time.Time{}
	}
	g.counts[s.Gesture]++
	g.seen[s.Gesture] = now
	g.last = &s
}

func (g galState) view(m Model, w, h int) box {
	gs := m.prof.Kit.Gestures()
	found := 0
	for _, x := range gs {
		if g.counts[x] > 0 {
			found++
		}
	}
	head := text(sTitle.Render("Every gesture CoinChord can hear with your kit") +
		sDim.Render(fmt.Sprintf("   you found %d of %d. Try them all: each one is a new word.", found, len(gs))))
	var cards []box
	for _, x := range gs {
		c := srcColor(x.Src)
		st := sPanel.Width(15)
		n := g.counts[x]
		label := sDim.Render("not yet")
		if n > 0 {
			label = sOK.Render(fmt.Sprintf("✔ heard %d×", n))
		}
		if t, ok := g.seen[x]; ok && m.now.Sub(t) < glow {
			st = st.BorderForeground(c).Background(lipgloss.Color("#262626"))
			label = lipgloss.NewStyle().Foreground(c).Bold(true).Render("★ DETECTED!")
		}
		if g.sel != nil && *g.sel == x {
			st = st.BorderForeground(cAccent)
		}
		cards = append(cards, wrapClick(st, text(art(x.Shape, c)+"\n"+chip(x)+"\n"+label), galPick{x}))
	}
	gridW := w - 44
	all := grid(gridW, 0, cards...)
	// scroll by card rows (each card is 7 lines)
	lines := strings.Split(all.s, "\n")
	skip := min(g.top, max(0, len(lines)-(h-2))) / 7 * 7
	visible := box{strings.Join(lines[skip:min(len(lines), skip+max(7, h-2))], "\n"), nil}
	for _, z := range all.zones {
		if z.y >= skip {
			z.y -= skip
			visible.zones = append(visible.zones, z)
		}
	}
	return vjoin(head, hjoin(1, visible, g.detail(m)))
}

// detail explains the selected gesture and why the last stroke was classified as it was.
func (g galState) detail(m Model) box {
	var parts []box
	if g.sel != nil {
		x := *g.sel
		parts = append(parts,
			text(chip(x)),
			text(sourceWords(m, x.Src)),
			text(lipgloss.NewStyle().Width(38).Render(x.Shape.HowTo())),
			text(""))
	} else {
		parts = append(parts, text(sDim.Render("Click a card for how-to.")), text(""))
	}
	parts = append(parts, text(why(m, g.last)))
	return wrap(sPanel.Width(42), vjoin(parts...))
}

// why shows each measured value of the last stroke against its knob.
func why(m Model, s *audio.Stroke) string {
	if s == nil {
		return sDim.Render("Perform any gesture to see\nhow CoinChord measured it.")
	}
	t, x := m.prof.Tuning, s.Metrics
	row := func(name string, v, scale, mark float64, note string) string {
		return fmt.Sprintf("%-9s %s %s", name, bar(v, scale, 14, cAccent, mark/scale), sDim.Render(note))
	}
	lines := []string{
		sBold.Render("Last stroke: ") + chip(s.Gesture),
		row("duration", x.DurMs, t.SlideMs*1.5, t.TapMaxMs, fmt.Sprintf("%.0fms │tap≤%.0f", x.DurMs, t.TapMaxMs)),
		row("loudness", x.Peak, t.TapPeak*t.KnockRatio*1.5, t.TapPeak*t.KnockRatio, fmt.Sprintf("%.2f │knock", x.Peak)),
		row("friction", x.Flat, 1, t.ScrapeFlat, fmt.Sprintf("%.2f │drag", x.Flat)),
		row("bumps", float64(x.Peaks), t.ZigzagPeaks*1.5, t.CirclePeaks, fmt.Sprintf("%d │circle", x.Peaks)),
		row("bend", x.DriftHz, t.CurveDriftHz*2, t.CurveDriftHz, fmt.Sprintf("%.0fHz │curve", x.DriftHz)),
		"",
		sBold.Render("Which striker?"),
	}
	for i, v := range m.prof.Kit.Voices() {
		mark := " "
		if i == s.Match.Best {
			mark = "◀"
		}
		lines = append(lines, fmt.Sprintf("%-3s %s %s %s", v.Striker, bar(s.Match.Sims[i], 1, 14, handColor(v.Striker.Hand()), -1), pct(s.Match.Sims[i]), mark))
	}
	if s.Match.Partner >= 0 {
		lines = append(lines, sDim.Render(fmt.Sprintf("mix: %.0f%% %s · two hands below %.0f%%",
			s.Match.Share*100, m.prof.Kit.Voices()[s.Match.Best].Striker, (0.5+t.BothMargin)*100)))
	}
	return strings.Join(lines, "\n")
}
