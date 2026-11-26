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
	np.Tuning, np.Records = p.Tuning, p.Records
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
