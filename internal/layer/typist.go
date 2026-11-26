package layer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Typist applies expansions to a text buffer. It capitalizes sentence
// starts, joins punctuation to the previous word and keeps an undo stack.
type Typist struct {
	Active  string   // active layer name
	Host    []string // actions only the host can do (shortcuts, mouse, commands), newest last
	text    string
	history []string
}

// Text returns the typed text.
func (t *Typist) Text() string { return t.text }

// Reset clears the text and history.
func (t *Typist) Reset() { t.text, t.history, t.Host = "", nil, nil }

const historyMax = 64

// Apply runs every action of e.
func (t *Typist) Apply(e Expansion) {
	before := t.text
	undo := false
	for _, a := range e.acts {
		switch a := a.(type) {
		case Text:
			t.typeText(string(a))
		case Combo:
			t.press(a)
		case Mouse:
			t.Host = append(t.Host, a.String())
		case Command:
			t.Host = append(t.Host, "do:"+a.String())
		case Switch:
			t.Active = string(a)
		case Undo:
			undo = true
			if n := len(t.history); n > 0 {
				t.text, t.history = t.history[n-1], t.history[:n-1]
			}
		}
	}
	if !undo && t.text != before {
		t.history = append(t.history, before)
		if len(t.history) > historyMax {
			t.history = t.history[1:]
		}
	}
}

func (t *Typist) typeText(s string) {
	if s == "" {
		return
	}
	if strings.ContainsRune(".,!?;:", rune(s[0])) {
		t.text = strings.TrimRight(t.text, " ")
	}
	if t.sentenceStart() {
		r, n := utf8.DecodeRuneInString(s)
		s = string(unicode.ToUpper(r)) + s[n:]
	}
	t.text += s
}

func (t *Typist) sentenceStart() bool {
	trim := strings.TrimRight(t.text, " ")
	last, _ := utf8.DecodeLastRuneInString(trim)
	return trim == "" || strings.ContainsRune(".!?\n¡¿", last)
}

func (t *Typist) press(c Combo) {
	if c.Mods != 0 {
		t.Host = append(t.Host, c.String())
		return
	}
	switch c.Key {
	case "enter":
		t.text = strings.TrimRight(t.text, " ") + "\n"
	case "tab":
		t.text += "\t"
	case "space":
		t.text += " "
	case "bs":
		_, n := utf8.DecodeLastRuneInString(t.text)
		t.text = t.text[:len(t.text)-n]
	default:
		if utf8.RuneCountInString(c.Key) == 1 {
			t.typeText(c.Key)
			return
		}
		t.Host = append(t.Host, c.String())
	}
}
