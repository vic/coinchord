// Package tutor holds practice stories, plans the chords to type them and
// keeps typing statistics.
package tutor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"coinchord/internal/gesture"
	"coinchord/internal/layer"
)

// Story is a practice text.
type Story struct {
	Title, Text string
}

// Builtin stories: short, neutral, easy English.
var Builtin = []Story{
	{"The Little Fox", "Once upon a time, a little fox lived in a green wood. The fox had a red ball. One day the ball rolled down the hill. The fox ran after it. A kind owl saw the fox. The owl said that she could help. She flew up and looked all around. The ball was by the river. The fox ran to the river and found the ball. The fox was so happy. The owl and the fox played with the ball all day. The end."},
	{"The Small Boat", "A small boat lived by the sea. Every day it watched the big ships go out. One morning the wind was soft and the sky was blue. The small boat went out to sea. It saw fish and birds and a big white cloud. When the sun went down, the boat came home. It was tired and very happy."},
	{"The Sleepy Moon", "The moon was sleepy. It had been up all night. A little star asked the moon to play. The moon said that it had to rest. So the star sang a soft song. The moon closed its eyes and smiled. Good night, moon."},
}

// Load reads every .txt file in dir as a story. A missing dir gives no stories.
func Load(dir string) ([]Story, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return nil, err
	}
	var out []Story
	for _, p := range paths {
		s, err := LoadFile(p)
		if err != nil {
			return out, err
		}
		out = append(out, s)
	}
	return out, nil
}

// LoadFile reads one text file as a story.
func LoadFile(path string) (Story, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Story{}, err
	}
	text := strings.Join(strings.Fields(string(b)), " ")
	if text == "" {
		return Story{}, fmt.Errorf("%s: empty text", path)
	}
	return Story{strings.TrimSuffix(filepath.Base(path), ".txt"), text}, nil
}

var sentenceEnd = regexp.MustCompile(`[.!?]+\s+`)

// Sentences splits the story; each sentence keeps its end mark and one trailing space.
func (s Story) Sentences() []string {
	var out []string
	rest := strings.Join(strings.Fields(s.Text), " ") + " "
	for rest != "" {
		end := len(rest)
		if loc := sentenceEnd.FindStringIndex(rest); loc != nil {
			end = loc[1]
		}
		out = append(out, strings.TrimSpace(rest[:end])+" ")
		rest = rest[end:]
	}
	return out
}

// Words returns the distinct words of all stories.
func Words(ss []Story) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range ss {
		for _, w := range strings.Fields(s.Text) {
			if !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}

// Step is one chord to perform.
type Step struct {
	Chord  gesture.Chord
	Layer  string // layer active when the chord is performed
	Expand layer.Expansion
	Text   string // typed text after the step
}

// Plan finds a chord sequence that types target, starting in layer start.
// It stays in start when it can, and switches to other layers to spell
// what start cannot type. Case differences are accepted.
func Plan(s *layer.Set, k gesture.Kit, start, target string) ([]Step, error) {
	p := newPlanner(s, k)
	ty := layer.Typist{Active: start}
	var steps []Step
	for !done(ty.Text(), target) {
		c, e, ok := p.best(ty, target)
		if !ok {
			return steps, fmt.Errorf("no binding can type the rest of %q after %q", target, ty.Text())
		}
		from := ty.Active
		ty.Apply(e)
		steps = append(steps, Step{c, from, e, ty.Text()})
		if len(steps) > 4*len(target)+8 {
			return steps, fmt.Errorf("plan too long for %q", target)
		}
	}
	// return to the start layer at the end, so the next sentence starts there
	if ty.Active != start {
		if c, e, ok := p.switchTo(start); ok {
			steps = append(steps, Step{c, ty.Active, e, ty.Text()})
		}
	}
	return steps, nil
}

func done(got, target string) bool {
	return strings.EqualFold(strings.TrimRight(got, " "), strings.TrimRight(target, " "))
}

// score rates how far ty has typed target after e: 2 per content byte,
// 1 for a trailing space; -1 when the text leaves the target.
func score(ty layer.Typist, e layer.Expansion, target string) int {
	ty.Apply(e)
	got := ty.Text()
	if done(got, target) {
		return 2 * len(target)
	}
	// a trailing space may be eaten by the next punctuation mark ("fox " + ". ")
	g := strings.TrimRight(got, " ")
	if len(g) >= len(target) || !strings.EqualFold(g, target[:len(g)]) {
		return -1
	}
	if len(got)-len(g) > 1 || (g != got && !strings.ContainsRune(" .,!?;:", rune(target[len(g)]))) {
		return -1
	}
	return 2*len(g) + len(got) - len(g)
}

// planner holds each layer's reachable bindings in ease order.
type planner struct {
	s     *layer.Set
	order map[string][]gesture.Chord // "" = global layer
}

func newPlanner(s *layer.Set, k gesture.Kit) planner {
	p := planner{s, map[string][]gesture.Chord{}}
	r := k.Ranker()
	reach := func(l layer.Layer) []gesture.Chord {
		return slices.DeleteFunc(l.Sorted(k), func(c gesture.Chord) bool { _, ok := r.Key(c); return !ok })
	}
	p.order[""] = reach(s.Global)
	for _, l := range s.Layers {
		p.order[l.Name] = reach(l)
	}
	return p
}

// best picks the binding with the most progress in the active layer, or a
// switch to the first layer that can progress.
func (p planner) best(ty layer.Typist, target string) (gesture.Chord, layer.Expansion, bool) {
	now := score(ty, layer.Expansion{}, target)
	pick := func(name string) (gesture.Chord, layer.Expansion, int) {
		var bc gesture.Chord
		var be layer.Expansion
		bp := now
		l, ok := p.s.Find(name)
		if !ok {
			return bc, be, bp
		}
		for _, c := range p.order[name] {
			if sc := score(ty, l.Bind[c], target); sc > bp {
				bc, be, bp = c, l.Bind[c], sc
			}
		}
		return bc, be, bp
	}
	if c, e, sc := pick(ty.Active); sc > now {
		return c, e, true
	}
	for _, l := range p.s.Layers {
		if _, _, sc := pick(l.Name); l.Name != ty.Active && sc > now {
			return p.switchTo(l.Name)
		}
	}
	return gesture.Chord{}, layer.Expansion{}, false
}

func (p planner) switchTo(name string) (gesture.Chord, layer.Expansion, bool) {
	for _, c := range p.order[""] {
		e := p.s.Global.Bind[c]
		for _, a := range e.Actions() {
			if sw, ok := a.(layer.Switch); ok && string(sw) == name {
				return c, e, true
			}
		}
	}
	return gesture.Chord{}, layer.Expansion{}, false
}

// Stats measure one practice run.
type Stats struct {
	Start   time.Time
	End     time.Time
	Chars   int // characters typed by correct chords
	Correct int
	Wrong   int
	Misses  map[string]int // expected chord -> wrong attempts
}

// NewStats starts a run at t.
func NewStats(t time.Time) Stats { return Stats{Start: t, End: t, Misses: map[string]int{}} }

// Hit records a correct chord that typed n characters.
func (s *Stats) Hit(n int, t time.Time) { s.Correct++; s.Chars += n; s.End = t }

// Miss records a wrong chord while want was expected.
func (s *Stats) Miss(want gesture.Chord, t time.Time) {
	s.Wrong++
	s.Misses[want.String()]++
	s.End = t
}

func (s Stats) minutes() float64 { return max(s.End.Sub(s.Start).Minutes(), 1.0/600) }

// WPM is words per minute, one word = 5 characters.
func (s Stats) WPM() float64 { return float64(s.Chars) / 5 / s.minutes() }

// CPM is correct chords per minute.
func (s Stats) CPM() float64 { return float64(s.Correct) / s.minutes() }

// Accuracy is the share of correct chords, 0..1.
func (s Stats) Accuracy() float64 {
	return float64(s.Correct) / max(1, float64(s.Correct+s.Wrong))
}

// Record is the saved summary of one finished run.
type Record struct {
	Story    string    `json:"story"`
	Lesson   int       `json:"lesson"`
	When     time.Time `json:"when"`
	WPM      float64   `json:"wpm"`
	CPM      float64   `json:"cpm"`
	Accuracy float64   `json:"accuracy"`
	Seconds  float64   `json:"seconds"`
}

// Record summarizes the run.
func (s Stats) Record(story string, lesson int) Record {
	return Record{story, lesson, s.End, s.WPM(), s.CPM(), s.Accuracy(), s.End.Sub(s.Start).Seconds()}
}
