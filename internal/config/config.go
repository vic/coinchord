// Package config loads and saves the user profile.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"coinchord/internal/gesture"
	"coinchord/internal/kits"
	"coinchord/internal/layer"
	"coinchord/internal/tutor"
)

// Score counts gesture test results.
type Score struct {
	Hits  int `json:"hits"`
	Tries int `json:"tries"`
}

// Profile is everything CoinChord remembers about one person.
type Profile struct {
	Preset     string           `json:"preset"`
	Kit        gesture.Kit      `json:"kit"`
	Tuning     gesture.Tuning   `json:"tuning"`
	Calibrated bool             `json:"calibrated"`
	Quality    map[string]Score `json:"quality"` // gesture name -> test results
	Layers     layer.Set        `json:"layers"`
	Records    []tutor.Record   `json:"records"`
	Setups     []Setup          `json:"setups"` // saved kits with their calibration and layers
}

// Setup is a saved kit: strikers, calibration, test scores and layers.
type Setup struct {
	Name       string           `json:"name"`
	Preset     string           `json:"preset"`
	Kit        gesture.Kit      `json:"kit"`
	Tuning     gesture.Tuning   `json:"tuning"`
	Calibrated bool             `json:"calibrated"`
	Quality    map[string]Score `json:"quality"`
	Layers     layer.Set        `json:"layers"`
}

// SaveSetup stores the active kit under name, replacing a setup of the same name.
func (p Profile) SaveSetup(name string) (Profile, error) {
	if strings.TrimSpace(name) == "" {
		return p, fmt.Errorf("setup needs a name")
	}
	b, err := json.Marshal(Setup{name, p.Preset, p.Kit, p.Tuning, p.Calibrated, p.Quality, p.Layers})
	if err != nil {
		return p, err
	}
	var s Setup // deep copy, so later edits of the active kit do not change the saved one
	if err := json.Unmarshal(b, &s); err != nil {
		return p, err
	}
	p.Setups = slices.DeleteFunc(slices.Clone(p.Setups), func(x Setup) bool { return x.Name == name })
	p.Setups = append(p.Setups, s)
	return p, nil
}

// UseSetup makes saved setup i the active kit.
func (p Profile) UseSetup(i int) (Profile, error) {
	if i < 0 || i >= len(p.Setups) {
		return p, fmt.Errorf("no setup %d", i)
	}
	b, err := json.Marshal(p.Setups[i])
	if err != nil {
		return p, err
	}
	var s Setup
	if err := json.Unmarshal(b, &s); err != nil {
		return p, err
	}
	p.Preset, p.Kit, p.Tuning, p.Calibrated, p.Quality, p.Layers = s.Name, s.Kit, s.Tuning.Clamp(), s.Calibrated, s.Quality, s.Layers
	if p.Quality == nil {
		p.Quality = map[string]Score{}
	}
	return p, nil
}

// DropSetup deletes saved setup i.
func (p Profile) DropSetup(i int) Profile {
	if i >= 0 && i < len(p.Setups) {
		p.Setups = slices.Delete(slices.Clone(p.Setups), i, i+1)
	}
	return p
}

// Dir is the CoinChord config directory.
func Dir() (string, error) {
	d, err := os.UserConfigDir()
	return filepath.Join(d, "coinchord"), err
}

// ForPreset builds a fresh profile for a kit preset.
func ForPreset(p kits.Preset) (Profile, error) {
	k, err := p.Kit()
	if err != nil {
		return Profile{}, err
	}
	set, err := layer.Defaults(k, tutor.Words(tutor.Builtin))
	if err != nil {
		return Profile{}, err
	}
	return Profile{Preset: p.Name, Kit: k, Tuning: gesture.DefaultTuning(), Quality: map[string]Score{}, Layers: set}, nil
}

// WithPreset switches the kit. Layers are kept when the strikers stay the
// same; otherwise they reset to the defaults of the new kit.
func (p Profile) WithPreset(pr kits.Preset) (Profile, error) {
	np, err := ForPreset(pr)
	if err != nil {
		return p, err
	}
	np.Tuning, np.Records, np.Setups = p.Tuning, p.Records, p.Setups
	if slices.Equal(codes(p.Kit), codes(np.Kit)) {
		np.Layers = p.Layers
	}
	return np, nil
}

func codes(k gesture.Kit) []string {
	var out []string
	for _, v := range k.Voices() {
		out = append(out, v.Striker.String())
	}
	return out
}

// Load reads the profile at path. A missing file gives the default profile.
// A broken file is moved to path+".bak" and reported, so nothing is lost.
func Load(path string) (Profile, error) {
	def, err := ForPreset(kits.Presets[0])
	if err != nil {
		return def, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return def, nil
	}
	if err != nil {
		return def, err
	}
	p := def
	if err := json.Unmarshal(b, &p); err != nil {
		bak := path + ".bak"
		if rerr := os.Rename(path, bak); rerr != nil {
			return def, fmt.Errorf("profile %s is broken (%v) and could not be moved: %w", path, err, rerr)
		}
		return def, fmt.Errorf("profile was broken (%v); kept it as %s and started fresh", err, bak)
	}
	p.Tuning = p.Tuning.Clamp()
	if set, added, err := p.Layers.WithMissing(p.Kit); err == nil && len(added) > 0 {
		p.Layers = set // new built-in layers since this profile was saved
	}
	if p.Quality == nil {
		p.Quality = map[string]Score{}
	}
	return p, nil
}

// Save writes the profile atomically.
func Save(path string, p Profile) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
