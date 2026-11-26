// Package kits is the catalog of striker kits people can pick.
package kits

import (
	"coinchord/internal/gesture"
)

// Stars is a rating from 1 to 5.
type Stars uint8

// Preset is a kit idea from everyday objects. Pitches are first guesses;
// calibration replaces them with measured sound prints.
type Preset struct {
	Name, Surface, Feel string
	Contrast            Stars // how easy the mic tells the strikers apart
	Glide               Stars // how well lines, curves and circles sound
	Speed               Stars // how fast the hands can move
	Voices              []Spec
}

// Spec is one striker of a preset.
type Spec struct {
	Code   string  // striker code, for example "L" or "R3"
	Object string  // what to hold or use
	Hz     float64 // first pitch guess
}

// Kit builds a gesture kit with synthetic prints at the guessed pitches.
func (p Preset) Kit() (gesture.Kit, error) {
	vs := make([]gesture.Voice, 0, len(p.Voices))
	for _, s := range p.Voices {
		st, err := gesture.ParseStriker(s.Code)
		if err != nil {
			return gesture.Kit{}, err
		}
		vs = append(vs, gesture.Voice{Striker: st, Name: s.Object, Print: gesture.ToneAt(s.Hz)})
	}
	return gesture.NewKit(vs)
}

func nails(hand string, hz []float64) []Spec {
	fingers := []string{"thumb", "index", "middle", "ring", "pinky"}
	out := make([]Spec, len(hz))
	for i, h := range hz {
		out[i] = Spec{hand + string(rune('1'+i)), hand + " " + fingers[i] + " nail", h}
	}
	return out
}

// Presets lists the kits, simplest first.
var Presets = []Preset{
	{"Pocket coins", "Wood or laminate desk", "The gold standard. Smooth edges glide into circles and curves.", 4, 5, 4,
		[]Spec{{"L", "Small coin", 9200}, {"R", "Large coin", 5800}}},
	{"Key & spoon", "Any hard desk", "Maximum contrast. Works even in a noisy café.", 5, 3, 3,
		[]Spec{{"L", "House key", 10500}, {"R", "Metal spoon", 5200}}},
	{"Desk pens", "Desk or notebook cover", "Office native. Good if you fidget with pens.", 3, 3, 4,
		[]Spec{{"L", "Plastic pen cap", 8800}, {"R", "Metal pen", 6000}}},
	{"Nail & ring", "Any hard surface", "Zero objects. Always with you.", 3, 2, 5,
		[]Spec{{"L", "Fingernail flick", 11000}, {"R", "Ring or knuckle", 4800}}},
	{"Pick & mug", "Ceramic mug rim, wooden coaster", "Ultra fast taps and sweeps, no hand tension.", 4, 4, 5,
		[]Spec{{"L", "Guitar pick", 9800}, {"R", "Mug rim or coaster", 5000}}},
	{"Kitchen four", "Wood table", "Four voices: twice the chords. Needs a careful calibration.", 3, 3, 3,
		[]Spec{{"L1", "Fork tine", 12000}, {"L2", "House key", 9000}, {"R1", "Bottle cap", 7200}, {"R2", "Spoon", 4800}}},
	{"Desk four", "Desk", "Four voices from the drawer.", 3, 3, 3,
		[]Spec{{"L1", "Paperclip", 12500}, {"L2", "Pen cap", 8800}, {"R1", "Coin", 6800}, {"R2", "Binder clip", 4600}}},
	{"Ten nails", "Laptop palm rest", "No objects at all. Each nail is a voice: the most chords, the hardest to separate.", 2, 3, 5,
		append(nails("L", []float64{6000, 9500, 10500, 11500, 12500}), nails("R", []float64{5500, 8000, 8800, 9800, 10800})...)},
	{"Fingertip drums", "Laptop palm rest", "Nail tip and finger pad per hand: click versus thud.", 3, 2, 5,
		[]Spec{{"L1", "Left nail tip", 11000}, {"L2", "Left finger pad", 3500}, {"R1", "Right nail tip", 9500}, {"R2", "Right knuckle", 4500}}},
}
