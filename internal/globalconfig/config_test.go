package globalconfig_test

import (
	"path/filepath"
	"testing"

	"github.com/sebastianstupak/m2a/internal/globalconfig"
)

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := globalconfig.LoadFrom(filepath.Join(dir, "m2a.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roots) != 0 {
		t.Fatalf("expected empty roots, got %v", cfg.Roots)
	}
}

func TestAddRemoveRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m2a.yml")

	cfg, _ := globalconfig.LoadFrom(path)
	cfg.AddRoot(`C:\projects`)
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}

	cfg2, err := globalconfig.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Roots) != 1 || cfg2.Roots[0] != `C:\projects` {
		t.Fatalf("expected [C:\\projects], got %v", cfg2.Roots)
	}

	cfg2.RemoveRoot(`C:\projects`)
	if err := cfg2.SaveTo(path); err != nil {
		t.Fatal(err)
	}

	cfg3, _ := globalconfig.LoadFrom(path)
	if len(cfg3.Roots) != 0 {
		t.Fatalf("expected empty after remove, got %v", cfg3.Roots)
	}
}

func TestAddRootIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m2a.yml")
	cfg, _ := globalconfig.LoadFrom(path)
	cfg.AddRoot(`C:\projects`)
	cfg.AddRoot(`C:\projects`)
	if len(cfg.Roots) != 1 {
		t.Fatalf("expected 1, got %d", len(cfg.Roots))
	}
}

func TestRemoveRootPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m2a.yml")
	cfg, _ := globalconfig.LoadFrom(path)
	cfg.AddRoot(`C:\a`)
	cfg.AddRoot(`C:\b`)
	cfg.AddRoot(`C:\c`)
	cfg.RemoveRoot(`C:\b`)
	if len(cfg.Roots) != 2 || cfg.Roots[0] != `C:\a` || cfg.Roots[1] != `C:\c` {
		t.Fatalf("unexpected roots after remove: %v", cfg.Roots)
	}
}

func TestSaveToCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "m2a.yml")
	cfg, _ := globalconfig.LoadFrom(path)
	cfg.AddRoot(`C:\projects`)
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo with nested dirs failed: %v", err)
	}
	cfg2, err := globalconfig.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Roots) != 1 {
		t.Fatalf("expected 1 root, got %v", cfg2.Roots)
	}
}
