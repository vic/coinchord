package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// zone is a clickable rectangle that sends msg when clicked.
type zone struct {
	x, y, w, h int
	msg        tea.Msg
}

// box is rendered text plus the click zones inside it, relative to its top-left.
type box struct {
	s     string
	zones []zone
}

func text(s string) box { return box{s: s} }

// click makes the whole box send msg when clicked.
func click(b box, msg tea.Msg) box {
	b.zones = append(b.zones, zone{0, 0, lipgloss.Width(b.s), lipgloss.Height(b.s), msg})
	return b
}

func (b box) shift(dx, dy int) []zone {
	out := make([]zone, len(b.zones))
	for i, z := range b.zones {
		z.x, z.y = z.x+dx, z.y+dy
		out[i] = z
	}
	return out
}

// hjoin places boxes side by side, top aligned, with gap spaces between.
func hjoin(gap int, bs ...box) box {
	var out box
	parts := make([]string, 0, 2*len(bs))
	x := 0
	for i, b := range bs {
		if i > 0 && gap > 0 {
			parts = append(parts, strings.Repeat(" ", gap))
			x += gap
		}
		parts = append(parts, b.s)
		out.zones = append(out.zones, b.shift(x, 0)...)
		x += lipgloss.Width(b.s)
	}
	out.s = lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return out
}

// vjoin stacks boxes, left aligned.
func vjoin(bs ...box) box {
	var out box
	parts := make([]string, 0, len(bs))
	y := 0
	for _, b := range bs {
		parts = append(parts, b.s)
		out.zones = append(out.zones, b.shift(0, y)...)
		y += lipgloss.Height(b.s)
	}
	out.s = lipgloss.JoinVertical(lipgloss.Left, parts...)
	return out
}

// wrap renders b inside a style (border, padding, margin) keeping zones aligned.
// Do not use center alignment on st: zones would drift.
func wrap(st lipgloss.Style, b box) box {
	dx := st.GetMarginLeft() + st.GetBorderLeftSize() + st.GetPaddingLeft()
	dy := st.GetMarginTop() + st.GetBorderTopSize() + st.GetPaddingTop()
	return box{st.Render(b.s), b.shift(dx, dy)}
}

// wrapClick is wrap plus a zone over the whole result.
func wrapClick(st lipgloss.Style, b box, msg tea.Msg) box {
	w := wrap(st, b)
	return box{w.s, append([]zone{{0, 0, lipgloss.Width(w.s), lipgloss.Height(w.s), msg}}, w.zones...)}
}

// hit returns the message of the innermost zone at x,y.
func (b box) hit(x, y int) (tea.Msg, bool) {
	for i := len(b.zones) - 1; i >= 0; i-- {
		z := b.zones[i]
		if x >= z.x && x < z.x+z.w && y >= z.y && y < z.y+z.h {
			return z.msg, true
		}
	}
	return nil, false
}

// grid lays boxes in rows that fit width.
func grid(width, gap int, bs ...box) box {
	var rows []box
	var row []box
	w := 0
	for _, b := range bs {
		bw := lipgloss.Width(b.s)
		if len(row) > 0 && w+gap+bw > width {
			rows = append(rows, hjoin(gap, row...))
			row, w = nil, 0
		}
		if len(row) > 0 {
			w += gap
		}
		row = append(row, b)
		w += bw
	}
	if len(row) > 0 {
		rows = append(rows, hjoin(gap, row...))
	}
	return vjoin(rows...)
}
