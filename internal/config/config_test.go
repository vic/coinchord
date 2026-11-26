package config

import (
	"os"
	"path/filepath"
	"testing"

	"coinchord/internal/kits"
)

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	p, err := ForPreset(kits.Presets[7]) // ten nails
	if err != nil {
		t.Fatal(err)
	}
	p.Tuning.Trigger = 99 // out of range: must clamp on load
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.Preset != p.Preset || len(q.Kit.Voices()) != 10 || q.Tuning.Trigger != 0.3 {
		t.Fatalf("got %s %d voices trigger %v", q.Preset, len(q.Kit.Voices()), q.Tuning.Trigger)
	}
}

func TestBrokenProfileKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(path, []byte(`{"kit":[{"striker":"L9"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("want error for bad striker")
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}
