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
	if q.Preset != p.Preset || len(q.Kit.Voices()) != 10 || q.Tuning.Trigger != 1 {
		t.Fatalf("got %s %d voices trigger %v", q.Preset, len(q.Kit.Voices()), q.Tuning.Trigger)
	}
}

func TestBrokenProfileKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(path, []byte(`{"kit":[{"striker":"L100"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("want error for bad striker")
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestSetups(t *testing.T) {
	p, err := ForPreset(kits.Presets[0])
	if err != nil {
		t.Fatal(err)
	}
	p.Tuning.Trigger = 0.05
	p, err = p.SaveSetup("desk")
	if err != nil {
		t.Fatal(err)
	}
	p, _ = p.WithPreset(kits.Presets[7])
	p.Tuning.Trigger = 0.2
	if len(p.Setups) != 1 || p.Setups[0].Tuning.Trigger != 0.05 {
		t.Fatal("saved setup must survive kit changes unchanged")
	}
	p, err = p.UseSetup(0)
	if err != nil || len(p.Kit.Voices()) != 2 || p.Tuning.Trigger != 0.05 || p.Preset != "desk" {
		t.Fatalf("use: %v %d %v %s", err, len(p.Kit.Voices()), p.Tuning.Trigger, p.Preset)
	}
	if _, err := p.SaveSetup(" "); err == nil {
		t.Fatal("blank name must fail")
	}
	if p = p.DropSetup(0); len(p.Setups) != 0 {
		t.Fatal("drop")
	}
}
