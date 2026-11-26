package gesture

import (
	"encoding/json"
	"fmt"
	"math"

	"coinchord/internal/dsp"
)

// NB is the number of log-spaced spectral bands in a sound print.
const NB = 32

// Print is a unit-length vector of band energies: the sound print of a striker.
type Print [NB]float64

var bandEdges = func() (e [NB + 1]float64) {
	for i := range e {
		e[i] = dsp.MinHz * math.Pow(dsp.MaxHz/dsp.MinHz, float64(i)/NB)
	}
	return e
}()

// BandRange is the frequency range of band i in Hz.
func BandRange(i int) (lo, hi float64) { return bandEdges[i], bandEdges[i+1] }

// BandCenter is the center frequency of band i in Hz.
func BandCenter(i int) float64 { return math.Sqrt(bandEdges[i] * bandEdges[i+1]) }

// addFrame adds the band energies of f to p (not normalized).
func (p *Print) addFrame(f dsp.Frame) {
	for i := range p {
		p[i] += f.Band(bandEdges[i], bandEdges[i+1])
	}
}

func (p Print) unit() Print {
	var n float64
	for _, v := range p {
		n += v * v
	}
	n = math.Sqrt(n) + 1e-12
	for i := range p {
		p[i] /= n
	}
	return p
}

func (p Print) dot(q Print) (s float64) {
	for i := range p {
		s += p[i] * q[i]
	}
	return s
}

// ToneAt is a synthetic print for a ring at hz, used before calibration.
func ToneAt(hz float64) Print {
	var p Print
	for i := range p {
		d := math.Log2(BandCenter(i)/hz) / 0.25
		p[i] = math.Exp(-d * d)
	}
	return p.unit()
}

// Average returns the unit mean of prints.
func Average(ps []Print) Print {
	var s Print
	for _, p := range ps {
		for i := range s {
			s[i] += p[i]
		}
	}
	return s.unit()
}

// Similarity is the cosine similarity 0..1 of two prints.
func Similarity(a, b Print) float64 { return a.dot(b) }

// Voice is one calibrated striker of a kit.
type Voice struct {
	Striker Striker `json:"striker"`
	Name    string  `json:"name"`
	Print   Print   `json:"print"`
}

// Kit is a non-empty set of voices with distinct strikers.
// Build it with NewKit; JSON decoding uses the same checks.
type Kit struct{ voices []Voice }

// NewKit checks the voices and builds a kit.
func NewKit(vs []Voice) (Kit, error) {
	if len(vs) == 0 {
		return Kit{}, fmt.Errorf("kit: needs at least one striker")
	}
	seen := map[Striker]bool{}
	for _, v := range vs {
		if seen[v.Striker] {
			return Kit{}, fmt.Errorf("kit: striker %v twice", v.Striker)
		}
		seen[v.Striker] = true
	}
	return Kit{append([]Voice(nil), vs...)}, nil
}

// Voices returns a copy of the voices.
func (k Kit) Voices() []Voice { return append([]Voice(nil), k.voices...) }

// WithPrint returns a kit where voice i has print p.
func (k Kit) WithPrint(i int, p Print) Kit {
	vs := k.Voices()
	vs[i].Print = p.unit()
	return Kit{vs}
}

// Name returns the object name of striker s, or its code.
func (k Kit) Name(s Striker) string {
	for _, v := range k.voices {
		if v.Striker == s {
			return v.Name
		}
	}
	return s.String()
}

func (k Kit) MarshalJSON() ([]byte, error) { return json.Marshal(k.voices) }

func (k *Kit) UnmarshalJSON(b []byte) error {
	var vs []Voice
	if err := json.Unmarshal(b, &vs); err != nil {
		return err
	}
	nk, err := NewKit(vs)
	*k = nk
	return err
}

// Sources lists solos, then duos of strikers on different hands.
func (k Kit) Sources() []Source {
	var solo, duo []Source
	for i, a := range k.voices {
		solo = append(solo, Solo(a.Striker))
		for _, b := range k.voices[i+1:] {
			if a.Striker.hand != b.Striker.hand {
				d, _ := Duo(a.Striker, b.Striker) // hands differ: checked above
				duo = append(duo, d)
			}
		}
	}
	return append(solo, duo...)
}

// Gestures lists every gesture of the kit in ease order.
func (k Kit) Gestures() []Gesture {
	var solo, duo []Gesture
	srcs := k.Sources()
	for _, sh := range Shapes {
		for _, s := range srcs {
			g := Gesture{s, sh}
			if s.IsDuo() {
				duo = append(duo, g)
			} else {
				solo = append(solo, g)
			}
		}
	}
	return append(solo, duo...)
}

// Ease lists all chords of the kit: singles, then pairs by summed rank.
func (k Kit) Ease() []Chord {
	all := k.Gestures()
	n := len(all)
	out := make([]Chord, 0, n*(n+1))
	for _, g := range all {
		out = append(out, Single(g))
	}
	for sum := 0; sum <= 2*(n-1); sum++ {
		for i := max(0, sum-n+1); i <= min(sum, n-1); i++ {
			out = append(out, Pair(all[i], all[sum-i]))
		}
	}
	return out
}

// Match explains which source a stroke matched.
type Match struct {
	Sims    []float64 // similarity per voice, same order as Voices
	Best    int       // index of the best voice
	Partner int       // best voice of the other hand, -1 when none
	Share   float64   // share of Best in a two-voice mix: 1 = solo, 0.5 = even duo
}

// match finds the source of a stroke print p.
func (k Kit) match(p Print, margin float64) (Source, Match) {
	m := Match{Sims: make([]float64, len(k.voices)), Partner: -1, Share: 1}
	for i, v := range k.voices {
		m.Sims[i] = p.dot(v.Print)
		if m.Sims[i] > m.Sims[m.Best] {
			m.Best = i
		}
	}
	best := k.voices[m.Best]
	for i, v := range k.voices {
		if v.Striker.hand != best.Striker.hand && (m.Partner < 0 || m.Sims[i] > m.Sims[m.Partner]) {
			m.Partner = i
		}
	}
	if m.Partner < 0 {
		return Solo(best.Striker), m
	}
	// least squares fit of p as wa*A + wb*B, weights clamped at 0
	part := k.voices[m.Partner]
	c := best.Print.dot(part.Print)
	sa, sb := m.Sims[m.Best], m.Sims[m.Partner]
	det := math.Max(1-c*c, 1e-9)
	wa, wb := math.Max(0, (sa-c*sb)/det), math.Max(0, (sb-c*sa)/det)
	m.Share = wa / (wa + wb + 1e-12)
	if m.Share > 0.5+margin {
		return Solo(best.Striker), m
	}
	d, _ := Duo(best.Striker, part.Striker) // hands differ by construction above
	return d, m
}

// Ranker orders chords like Ease without listing every chord.
type Ranker map[Gesture]int

// Ranker indexes the kit's gestures.
func (k Kit) Ranker() Ranker {
	r := Ranker{}
	for i, g := range k.Gestures() {
		r[g] = i
	}
	return r
}

// Key is the ease sort key of c; ok is false when c uses a gesture outside the kit.
func (r Ranker) Key(c Chord) (key [3]int, ok bool) {
	st := c.Steps()
	i, oka := r[st[0]]
	if len(st) == 1 {
		return [3]int{0, i, 0}, oka
	}
	j, okb := r[st[1]]
	return [3]int{1, i + j, i}, oka && okb
}
