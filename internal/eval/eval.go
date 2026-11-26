// Package eval scores the detector on a recorded session: raw audio plus
// the labels written while calibrating ("asked for X, heard Y at T").
package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"coinchord/internal/audio"
	"coinchord/internal/gesture"
)

// Label is one line of a labels file.
type Label struct {
	Ms   int64  `json:"ms"`
	Want string `json:"want"`
	Got  string `json:"got"`
}

// match is how far a replayed stroke may move from its label and still count.
const match = 250 * time.Millisecond

// Report is the outcome of a replay.
type Report struct {
	Labels, Hits, Missed int
	Confused             map[string]int // "want → got" counts
}

func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d labeled gestures detected right (%.0f%%), %d not detected at all\n",
		r.Hits, r.Labels, 100*float64(r.Hits)/float64(max(1, r.Labels)), r.Missed)
	keys := make([]string, 0, len(r.Confused))
	for k := range r.Confused {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return r.Confused[keys[i]] > r.Confused[keys[j]] })
	for _, k := range keys {
		fmt.Fprintf(&b, "  %3d× %s\n", r.Confused[k], k)
	}
	return b.String()
}

// ReadLabels parses a labels file (one JSON object per line).
func ReadLabels(r io.Reader) ([]Label, error) {
	var out []Label
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var l Label
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return out, fmt.Errorf("label line %d: %w", len(out)+1, err)
		}
		if _, err := gesture.ParseGesture(l.Want); err != nil {
			return out, err
		}
		out = append(out, l)
	}
	return out, sc.Err()
}

// Run replays raw audio through the detector and scores it against labels.
func Run(raw io.Reader, labels []Label, s audio.Setup) Report {
	var setup atomic.Pointer[audio.Setup]
	setup.Store(&s)
	var strokes []audio.Stroke
	audio.Listen(raw, &setup, func(e any) {
		if st, ok := e.(audio.Stroke); ok {
			strokes = append(strokes, st)
		}
	})
	rep := Report{Labels: len(labels), Confused: map[string]int{}}
	used := make([]bool, len(strokes))
	for _, l := range labels {
		at := time.Duration(l.Ms) * time.Millisecond
		best := -1
		for i, st := range strokes {
			d := max(st.At-at, at-st.At)
			if !used[i] && d <= match && (best < 0 || d < max(strokes[best].At-at, at-strokes[best].At)) {
				best = i
			}
		}
		if best < 0 {
			rep.Missed++
			continue
		}
		used[best] = true
		if got := strokes[best].Gesture.String(); got == l.Want {
			rep.Hits++
		} else {
			rep.Confused[l.Want+" → "+got]++
		}
	}
	return rep
}
