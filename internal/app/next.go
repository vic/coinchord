package app

import (
	"fmt"
	"strings"

	"coinchord/internal/audio"
)

const triedEnough = 6 // gestures heard before "play with gestures" counts as done

func (a *App) tried() int {
	n := len(a.heard)
	for _, s := range a.prof.Quality {
		if s.Tries > 0 {
			n++
		}
	}
	return n
}

func (a *App) hasRecord(drill bool) bool {
	for _, r := range a.prof.Records {
		if strings.HasPrefix(r.Story, "drill: ") == drill {
			return true
		}
	}
	return false
}

// nextJSON is the single thing to do now, with the command its button sends.
func (a *App) nextJSON() map[string]any {
	say := func(text, button string, cmds ...Cmd) map[string]any {
		return map[string]any{"say": text, "button": button, "cmds": cmds}
	}
	switch st := a.mon.State(); {
	case a.micErr != "":
		return say("CoinChord can't hear anything: "+a.micErr, "")
	case st == audio.MicSilent:
		return say("Your microphone is sending silence. It's probably muted.", "Unmute", Cmd{Do: "mic", I: int(audio.Unmute)})
	case st == audio.MicClipping:
		return say("Your microphone is too loud to hear taps. Turn it down a little.", "Turn down", Cmd{Do: "mic", I: int(audio.Quieter)})
	case !a.prof.Calibrated:
		return say("Teach CoinChord how your objects sound. It takes about two minutes.", "Calibrate", Cmd{Do: "page", Name: string(pageCalibrate)}, Cmd{Do: "cal.go", I: int(calQuiet)})
	case a.tried() < triedEnough:
		return say(fmt.Sprintf("Play with gestures and watch them light up (%d of %d tried).", a.tried(), triedEnough), "Gestures", Cmd{Do: "page", Name: string(pageGestures)})
	case !a.hasRecord(false):
		return say("Your first lesson: see one gesture type a whole phrase.", "Start lesson", Cmd{Do: "page", Name: string(pageTutor)}, Cmd{Do: "tut.lesson", I: 0})
	case !a.hasRecord(true):
		return say("Now drive your computer: switch windows with chords.", "Windows drill", Cmd{Do: "page", Name: string(pageTutor)}, Cmd{Do: "tut.drill", I: 0})
	}
	best := 0.0
	for _, r := range a.prof.Records {
		best = max(best, r.WPM)
	}
	return say(fmt.Sprintf("Your best is %.0f words per minute. Beat it!", best), "Practice", Cmd{Do: "page", Name: string(pageTutor)})
}

func (a *App) journeyJSON() []map[string]any {
	steps := []struct {
		name string
		done bool
		page page
	}{
		{"Pick a kit", a.prof.Calibrated || a.page != pageKits, pageKits},
		{"Calibrate", a.prof.Calibrated, pageCalibrate},
		{"Try gestures", a.tried() >= triedEnough, pageGestures},
		{"First lesson", a.hasRecord(false), pageTutor},
		{"Drive your computer", a.hasRecord(true), pageTutor},
	}
	var out []map[string]any
	for _, s := range steps {
		out = append(out, map[string]any{"name": s.name, "done": s.done, "page": s.page})
	}
	return out
}
