package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"coinchord/internal/dsp"
	"coinchord/internal/gesture"
)

var (
	cLeft   = lipgloss.Color("#5fd7ff")
	cRight  = lipgloss.Color("#ffaf5f")
	cDuo    = lipgloss.Color("#d787ff")
	cOK     = lipgloss.Color("#87d787")
	cWarn   = lipgloss.Color("#ffd75f")
	cBad    = lipgloss.Color("#ff8787")
	cDim    = lipgloss.Color("#6c6c6c")
	cText   = lipgloss.Color("#e4e4e4")
	cAccent = lipgloss.Color("#87afff")

	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sBold   = lipgloss.NewStyle().Bold(true).Foreground(cText)
	sOK     = lipgloss.NewStyle().Foreground(cOK)
	sWarn   = lipgloss.NewStyle().Foreground(cWarn)
	sBad    = lipgloss.NewStyle().Foreground(cBad)
	sPanel  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cDim).Padding(0, 1)
	sActive = sPanel.BorderForeground(cAccent)
	sBtn    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(0, 1).Foreground(cText)
	sBtnHot = sBtn.BorderForeground(cOK).Foreground(cOK).Bold(true)
	sChip   = lipgloss.NewStyle().Background(lipgloss.Color("#3a3a3a")).Foreground(cText).Bold(true)
)

// button is a bordered clickable label.
func button(label string, msg any) box { return click(text(sBtn.Render(label)), msg) }

// hotButton is the main action of a screen.
func hotButton(label string, msg any) box { return click(text(sBtnHot.Render(label)), msg) }

// link is a one-line clickable label.
func link(label string, msg any) box {
	return click(text(lipgloss.NewStyle().Foreground(cAccent).Underline(true).Render(label)), msg)
}

func handColor(h gesture.Hand) color.Color { return []color.Color{cLeft, cRight}[h] }

func srcColor(s gesture.Source) color.Color {
	if s.IsDuo() {
		return cDuo
	}
	return handColor(s.Strikers()[0].Hand())
}

// chip renders a gesture as a colored tag like "L ○ CIRC".
func chip(g gesture.Gesture) string {
	return lipgloss.NewStyle().Foreground(srcColor(g.Src)).Bold(true).
		Render(g.Src.String() + " " + g.Shape.Glyph() + " " + g.Shape.String())
}

// chordChip renders a chord as chips joined by "+".
func chordChip(c gesture.Chord) string {
	var parts []string
	for _, g := range c.Steps() {
		parts = append(parts, chip(g))
	}
	return strings.Join(parts, sDim.Render(" + "))
}

// shapeArt is a 3-line drawing of a shape, 9 cells wide.
var shapeArt = map[gesture.Shape][3]string{
	gesture.Tap:    {"    ↓    ", "    ●    ", "         "},
	gesture.Knock:  {"   ↓↓↓   ", "   ✱●✱   ", "         "},
	gesture.Line:   {"         ", "  ●───▶  ", "         "},
	gesture.Curve:  {"  ●──╮   ", "      │  ", "      ▼  "},
	gesture.Circle: {"  ╭───╮  ", "  │ ● │  ", "  ╰──◀╯  "},
	gesture.Slide:  {"         ", "●═════▶  ", "         "},
	gesture.Zigzag: {"         ", " ●╱╲╱╲▶  ", "         "},
}

func art(s gesture.Shape, c color.Color) string {
	a := shapeArt[s]
	return lipgloss.NewStyle().Foreground(c).Render(strings.Join(a[:], "\n"))
}

// flash is a recent detection on one striker.
type flash struct {
	shape gesture.Shape
	at    time.Time
}

const glow = 900 * time.Millisecond

// coins draws every striker of the kit; recent detections glow.
func coins(k gesture.Kit, fl map[gesture.Striker]flash, now time.Time) box {
	var left, right []box
	for _, v := range k.Voices() {
		c := handColor(v.Striker.Hand())
		f, ok := fl[v.Striker]
		inner := sDim.Render(" " + v.Striker.String() + " ")
		st := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cDim).Width(7).Align(lipgloss.Center)
		if ok && now.Sub(f.at) < glow {
			inner = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(c).
				Render(" " + f.shape.Glyph() + " " + f.shape.String() + " ")
			st = st.BorderForeground(c).Width(9)
		}
		name := lipgloss.NewStyle().Foreground(c).Width(11).Render(trunc(v.Name, 11))
		b := vjoin(text(st.Render(inner)), text(name))
		if v.Striker.Hand() == gesture.Left {
			left = append(left, b)
		} else {
			right = append(right, b)
		}
	}
	return hjoin(4, hjoin(1, left...), hjoin(1, right...))
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// bar draws v/max as a width-cell bar with an optional mark position (0..1, <0 = none).
func bar(v, max float64, width int, c color.Color, mark float64) string {
	fill := int(math.Round(math.Min(1, math.Max(0, v/max)) * float64(width)))
	cells := []rune(strings.Repeat("█", fill) + strings.Repeat("░", width-fill))
	if mark >= 0 {
		i := min(width-1, int(mark*float64(width)))
		cells[i] = '│'
	}
	return lipgloss.NewStyle().Foreground(c).Render(string(cells))
}

// level shows loudness against the trigger.
func level(rms, trigger float64) string {
	c := cDim
	if rms > trigger {
		c = cOK
	}
	scale := trigger * 4
	return bar(rms, scale, 24, c, trigger/scale)
}

var blocks = []rune(" ▁▂▃▄▅▆▇█")

// spectrum draws the band energies of a frame (rows high) and marks each voice's pitch.
func spectrum(f dsp.Frame, k gesture.Kit, trigger float64, rows int) string {
	var p [gesture.NB]float64
	// floor: a Hann-windowed sine at trigger loudness peaks near trigger*BlockSize/4,
	// so silence stays flat instead of showing its own noise scaled up.
	top := trigger * dsp.BlockSize / 4
	for i := range p {
		lo, hi := gesture.BandRange(i)
		p[i] = f.Band(lo, hi)
		top = math.Max(top, p[i])
	}
	lines := make([]string, rows)
	for r := range rows {
		var sb strings.Builder
		for i := range p {
			h := p[i] / (top + 1e-12) * float64(rows)
			cell := math.Min(1, math.Max(0, h-float64(rows-1-r)))
			sb.WriteRune(blocks[int(cell*8)])
			sb.WriteRune(blocks[int(cell*8)])
		}
		lines[r] = lipgloss.NewStyle().Foreground(cAccent).Render(sb.String())
	}
	marks := []rune(strings.Repeat(" ", 2*gesture.NB))
	for _, v := range k.Voices() {
		best := 0
		for i, x := range v.Print {
			if x > v.Print[best] {
				best = i
			}
		}
		marks[2*best] = []rune(v.Striker.String())[0]
	}
	axis := sDim.Render(fmt.Sprintf("%-*s%s", 2*gesture.NB-5, "1.8k", "18k"))
	return strings.Join(append(lines, sDim.Render(string(marks)), axis), "\n")
}

// stars renders n of 5 stars.
func stars(n int) string {
	return sWarn.Render(strings.Repeat("★", n)) + sDim.Render(strings.Repeat("☆", 5-n))
}

// pct renders a 0..1 share as a colored percentage.
func pct(v float64) string {
	st := sBad
	switch {
	case v >= 0.85:
		st = sOK
	case v >= 0.6:
		st = sWarn
	}
	return st.Render(fmt.Sprintf("%3.0f%%", v*100))
}
