package layer

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"coinchord/internal/gesture"
)

// Layer is a named set of bindings.
type Layer struct {
	Name string                      `json:"name"`
	Bind map[gesture.Chord]Expansion `json:"bind"`
}

// Set is the global layer (always active, checked first) plus switchable layers.
type Set struct {
	Global Layer   `json:"global"`
	Layers []Layer `json:"layers"`
}

// Find returns the layer with the given name.
func (s *Set) Find(name string) (*Layer, bool) {
	i := slices.IndexFunc(s.Layers, func(l Layer) bool { return l.Name == name })
	if i < 0 {
		return nil, false
	}
	return &s.Layers[i], true
}

// Lookup resolves a chord in the global layer, then in the active layer.
func (s *Set) Lookup(active string, c gesture.Chord) (Expansion, bool) {
	if e, ok := s.Global.Bind[c]; ok {
		return e, true
	}
	l, ok := s.Find(active)
	if !ok {
		return Expansion{}, false
	}
	e, ok := l.Bind[c]
	return e, ok
}

// Sorted returns the bindings of l in the kit's ease order, chords outside the kit last.
func (l Layer) Sorted(k gesture.Kit) []gesture.Chord {
	r := k.Ranker()
	out := slices.Collect(maps.Keys(l.Bind))
	slices.SortFunc(out, func(a, b gesture.Chord) int {
		ka, oka := r.Key(a)
		kb, okb := r.Key(b)
		switch {
		case oka != okb && oka:
			return -1
		case oka != okb:
			return 1
		case oka && ka != kb:
			return slices.Compare(ka[:], kb[:])
		}
		return strings.Compare(a.String(), b.String())
	})
	return out
}

// Reachable reports whether every gesture of c belongs to the kit.
func Reachable(k gesture.Kit, c gesture.Chord) bool {
	_, ok := k.Ranker().Key(c)
	return ok
}

// FreeChord returns the easiest kit chord not bound in the global layer or l.
func (s *Set) FreeChord(k gesture.Kit, l *Layer) (gesture.Chord, bool) {
	for _, c := range k.Ease() {
		_, g := s.Global.Bind[c]
		_, b := l.Bind[c]
		if !g && !b {
			return c, true
		}
	}
	return gesture.Chord{}, false
}

// Unused returns the easiest kit chord bound in no layer at all.
// Use it for a new layer switch, which must not shadow any binding.
func (s *Set) Unused(k gesture.Kit) (gesture.Chord, bool) {
	used := map[gesture.Chord]bool{}
	for c := range s.Global.Bind {
		used[c] = true
	}
	for _, l := range s.Layers {
		for c := range l.Bind {
			used[c] = true
		}
	}
	for _, c := range k.Ease() {
		if !used[c] {
			return c, true
		}
	}
	return gesture.Chord{}, false
}

// assign binds items to the easiest chords not in skip.
// pins bind (left, right) items to the solo gesture of the first striker of each hand.
type pins map[gesture.Shape][2]string

func (p pins) place(k gesture.Kit, l *Layer, skip map[gesture.Chord]Expansion) error {
	first := map[gesture.Hand]gesture.Striker{}
	for _, v := range k.Voices() {
		if _, ok := first[v.Striker.Hand()]; !ok {
			first[v.Striker.Hand()] = v.Striker
		}
	}
	for _, sh := range gesture.Shapes {
		for h, src := range p[sh] {
			s, ok := first[gesture.Hand(h)]
			c := gesture.Single(gesture.Gesture{Src: gesture.Solo(s), Shape: sh})
			if _, taken := skip[c]; !ok || src == "" || taken {
				continue
			}
			e, err := Parse(src)
			if err != nil {
				return fmt.Errorf("layer %s pin %q: %w", l.Name, src, err)
			}
			l.Bind[c] = e
		}
	}
	return nil
}

func assign(name string, ease []gesture.Chord, skip map[gesture.Chord]Expansion, items []string, p pins, k gesture.Kit) (Layer, error) {
	l := Layer{Name: name, Bind: map[gesture.Chord]Expansion{}}
	if err := p.place(k, &l, skip); err != nil {
		return l, err
	}
	i := 0
	for _, c := range ease {
		if i == len(items) {
			return l, nil
		}
		_, used := skip[c]
		_, pinned := l.Bind[c]
		if used || pinned {
			continue
		}
		e, err := Parse(items[i])
		if err != nil {
			return l, fmt.Errorf("layer %s item %q: %w", name, items[i], err)
		}
		l.Bind[c] = e
		i++
	}
	if i < len(items) {
		return l, fmt.Errorf("layer %s: kit has %d chords for %d items", name, len(ease), len(items))
	}
	return l, nil
}

// Defaults builds the built-in layers for a kit. extra adds words to the
// english layer (for example, every word of the built-in stories).
func Defaults(k gesture.Kit, extra []string) (Set, error) {
	ease := k.Ease()
	// Layer switches go to two-hand gestures: rare in words, distinct, easy to recall.
	// Order: two-hand singles (not taps), pairs led by a two-hand gesture, then any pair.
	rank := func(c gesture.Chord) int {
		st := c.Steps()
		switch {
		case len(st) == 1 && st[0].Src.IsDuo() && st[0].Shape != gesture.Tap:
			return 0
		case len(st) == 2 && st[0].Src.IsDuo():
			return 1
		case len(st) == 2:
			return 2
		}
		return 3
	}
	var sw []gesture.Chord
	for r := 0; r < 3 && len(sw) < len(builtin); r++ {
		for _, c := range ease {
			if rank(c) == r && len(sw) < len(builtin) {
				sw = append(sw, c)
			}
		}
	}
	var names []string
	for _, d := range builtin {
		names = append(names, "{layer:"+d.name+"}")
	}
	glob, err := assign("global", sw, nil, names, nil, k)
	if err != nil {
		return Set{}, err
	}
	s := Set{Global: glob}
	for _, d := range builtin {
		items := d.items
		if d.name == "english" {
			items = withWords(items, extra)
		}
		l, err := assign(d.name, ease, glob.Bind, items, d.pins, k)
		if err != nil {
			return Set{}, err
		}
		s.Layers = append(s.Layers, l)
	}
	return s, nil
}

// withWords appends "word " items for words not yet present.
func withWords(items, words []string) []string {
	out := slices.Clone(items)
	for _, w := range words {
		w = strings.ToLower(strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' }))
		if w != "" && !slices.Contains(out, w+" ") {
			out = append(out, w+" ")
		}
	}
	return out
}

var builtin = []struct {
	name  string
	items []string
	pins  pins
}{
	{"english", strings.Split("the |and |. |to |a |, |{undo}|of |I |once upon a time, |in |was |he |that |it |his |her |you |as |had |with |for |she |not |at |but |be |my |on |have |him |is |said |me |which |by |so |this |all |from |they |no |were |if |would |or |when |what |there |been |one |could |very |an |who |them |we |now |more |out |do |are |up |their |your |will |little |than |then |some |into |any |well |much |about |time |know |should |man |did |like |upon |such |never |only |good |how |before |other |see |must |am |own |come |down |say |after |think |made |might |being |! |? |{enter}|the end.|thank you |{bs}", "|"), nil},
	{"alpha", strings.Split("{space}|e|t|a|o|i|n|s|h|r|d|l|c|u|m|w|f|g|y|p|b|v|k|j|x|q|z|{bs}|{undo}", "|"), nil},
	{"num", strings.Split("0|1|2|3|4|5|6|7|8|9|.|,|{space}|{bs}|+|-|*|/|=|%|:", "|"), nil},
	{"sym", []string{". ", ", ", "!", "?", ":", ";", "'", "\"", "(", ")", "[", "]", "{{", "}", "<", ">", "/", "\\", "-", "_", "=", "+", "*", "&", "|", "#", "@", "$", "%", "^", "~", "`", "{bs}"}, nil},
	{"kbd", strings.Split("{enter}|{bs}|{space}|{tab}|{esc}|{left}|{right}|{up}|{down}|{home}|{end}|{del}|{ctrl+bs}|{ctrl+z}|{ctrl+c}|{ctrl+v}|{ctrl+x}|{ctrl+s}|{ctrl+a}|{ctrl+f}|{alt+tab}|{ctrl+w}|{ctrl+t}|{ctrl+shift+t}|{ctrl+tab}|{ctrl+shift+tab}|{super+left}|{super+right}|{super+up}|{super+down}|{ctrl+alt+left}|{ctrl+alt+right}|{super+q}|{super+enter}|{pgup}|{pgdn}", "|"), nil},
	// Host layers share one motion map: left hand = left or up, right hand =
	// right or down; tap = small step, line = up/down, slide = far, curve = page,
	// circle = scroll.
	{"nav", strings.Split("{shift+left}|{shift+right}|{shift+up}|{shift+down}|{do:select-all}|{do:copy}|{do:cut}|{do:paste}|{do:undo}|{do:redo}|{do:find}|{enter}|{bs}|{del}|{esc}|{tab}", "|"), pins{
		gesture.Tap: {"{left}", "{right}"}, gesture.Line: {"{up}", "{down}"}, gesture.Slide: {"{home}", "{end}"},
		gesture.Curve: {"{pgup}", "{pgdn}"}, gesture.Circle: {"{scroll:-3}", "{scroll:3}"},
		gesture.Knock: {"{do:word-left}", "{do:word-right}"}, gesture.Zigzag: {"{do:doc-start}", "{do:doc-end}"},
	}},
	{"window", strings.Split("{do:tab-new}|{do:fullscreen}|{do:back}|{do:forward}|{do:save}|{esc}|{enter}", "|"), pins{
		gesture.Tap: {"{do:switch-back}", "{do:switch}"}, gesture.Line: {"{do:maximize}", "{do:minimize}"},
		gesture.Slide: {"{do:snap-left}", "{do:snap-right}"}, gesture.Curve: {"{do:desktop-prev}", "{do:desktop-next}"},
		gesture.Circle: {"{do:tab-prev}", "{do:tab-next}"}, gesture.Knock: {"{do:close}", "{do:tab-close}"},
		gesture.Zigzag: {"{do:show-desktop}", "{do:launcher}"},
	}},
	{"mouse", strings.Split("{down:left}|{up:left}|{scroll:-3,0}|{scroll:3,0}|{move:-150,-150}|{move:150,-150}|{move:-150,150}|{move:150,150}", "|"), pins{
		gesture.Tap: {"{move:-15,0}", "{move:15,0}"}, gesture.Line: {"{move:0,-15}", "{move:0,15}"},
		gesture.Slide: {"{move:-150,0}", "{move:150,0}"}, gesture.Curve: {"{move:0,-150}", "{move:0,150}"},
		gesture.Circle: {"{scroll:-3}", "{scroll:3}"}, gesture.Knock: {"{click:left}", "{click:right}"},
		gesture.Zigzag: {"{double:left}", "{click:middle}"},
	}},
	{"spanish", strings.Split("de |la |que |el |en |y |. |a |, |los |se |{undo}|del |las |un |por |con |no |una |su |para |es |al |lo |como |más |o |pero |sus |le |ha |me |si |sin |sobre |este |ya |entre |cuando |todo |esta |ser |son |dos |también |fue |había |era |muy |años |hasta |desde |está |mi |porque |qué |sólo |han |yo |hay |vez |puede |todos |así |nos |ni |parte |tiene |él |uno |donde |bien |tiempo |mismo |ese |ahora |cada |vida |otro |después |te |otros |aunque |esa |eso |hace |otra |tan |siempre |día |tanto |ella |tres |sí |dijo |gran |menos |había una vez, |ñ|á|é|í|ó|ú|¿|¡|{enter}|{bs}", "|"), nil},
	{"python", strings.Split("def |return |()|:{enter}| = |if |else:{enter}|for | in |import |from |print(|)|self.|None|True|False|while |class |elif |try:{enter}|except |with | as |lambda |yield |pass|break|continue| and | or |not | is |[|]|{{|}|, |.|'|\"|{enter}|{tab}|{bs}|{undo}", "|"), nil},
	{"go", strings.Split("func |return |if |err != nil |{{{enter}|}{enter}| := |for |range |package |import |type |struct |interface |var |const |nil|string|int|error|fmt.|go |defer |chan |select |case |switch |default:|make(|append(|len(|)|(|, |.|{enter}|{tab}|{bs}|{undo}", "|"), nil},
}

// WithMissing adds the built-in layers that s lacks (by name) and a switch
// chord for each. Existing layers and bindings stay unchanged.
func (s Set) WithMissing(k gesture.Kit) (Set, []string, error) {
	def, err := Defaults(k, nil)
	if err != nil {
		return s, nil, err
	}
	var added []string
	for _, l := range def.Layers {
		if _, ok := s.Find(l.Name); ok {
			continue
		}
		sw, err := Parse("{layer:" + l.Name + "}")
		if err != nil {
			return s, added, err
		}
		nl := Layer{Name: l.Name, Bind: map[gesture.Chord]Expansion{}}
		taken := func(c gesture.Chord) bool {
			_, g := s.Global.Bind[c]
			return g
		}
		for c, e := range l.Bind {
			if !taken(c) {
				nl.Bind[c] = e
			}
		}
		s.Layers = append(s.Layers, nl)
		c, ok := s.Unused(k)
		if !ok {
			return s, added, fmt.Errorf("no free chord to switch to %s", l.Name)
		}
		s.Global.Bind[c] = sw
		added = append(added, l.Name)
	}
	return s, added, nil
}
