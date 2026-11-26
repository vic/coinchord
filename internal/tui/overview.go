package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"coinchord/internal/kits"
)

type pickKit int

func (m *Model) pickKit(i int) {
	p, err := m.prof.WithPreset(kits.Presets[i])
	if err != nil {
		m.notice = err.Error()
		return
	}
	p.Calibrated = false
	m.prof = p
	m.save()
	m.cal = calState{}
	m.notice = "Kit set to " + p.Preset + ". Next: calibrate it, so the mic learns how each object sounds."
}

var mindset = []string{
	"Stop spelling. Start conducting.",
	"",
	"A normal keyboard makes you hunt one letter at a time. CoinChord types whole",
	"words and phrases from one small gesture, like a drummer hits one beat.",
	"",
	"• Your laptop mic hears 44,100 samples a second. It tells your strikers",
	"  apart by their ring, and tells a tap, line, curve or circle apart by sound.",
	"• Hold a striker in each hand (or use your nails). Tap, drag, hook, swirl.",
	"• Two gestures close together make a chord. More gestures detected means",
	"  more chords, and more chords means more words at one motion.",
	"• Do not think \"w-i-t-h\". Teach your hands: this motion means \"with\".",
	"",
	"Path: pick a kit below → Calibrate → explore Gestures → Tutor → your own Layers.",
}

func (m Model) overview(w int) box {
	intro := wrap(sPanel.Width(min(w-2, 84)), text(sTitle.Render("The mindset")+"\n"+strings.Join(mindset, "\n")))
	var cards []box
	for i, p := range kits.Presets {
		k, err := p.Kit()
		if err != nil {
			continue
		}
		g := len(k.Gestures())
		var objs []string
		for _, v := range p.Voices {
			objs = append(objs, v.Code+" "+v.Object)
		}
		body := strings.Join([]string{
			sBold.Render(p.Name),
			sDim.Render(trunc(strings.Join(objs, ", "), 34)),
			sDim.Render("on: " + p.Surface),
			"contrast " + stars(int(p.Contrast)),
			"shapes   " + stars(int(p.Glide)),
			"speed    " + stars(int(p.Speed)),
			sOK.Render(fmt.Sprintf("%d gestures · %d chords", g, g+g*g)),
			lipgloss.NewStyle().Width(34).Render(sDim.Render(p.Feel)),
		}, "\n")
		st := sPanel.Width(38)
		if p.Name == m.prof.Preset {
			st = sActive.Width(38).BorderForeground(cOK)
			body = sOK.Render("✔ your kit") + "\n" + body
		}
		cards = append(cards, wrapClick(st, text(body), pickKit(i)))
	}
	hint := text(sDim.Render("Click a kit to use it. Hard surfaces (wood, glass, a hardcover book) ring best."))
	next := hotButton("Calibrate my kit →", tabMsg(tabCalibrate))
	return vjoin(intro, hjoin(2, hint, next), grid(w, 1, cards...))
}
