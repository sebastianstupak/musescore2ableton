package projectregistry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebastianstupak/m2a/internal/projectregistry"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsProjects(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "song-a", "m2a.yml"), "score: song-a.mscz\n")
	writeFile(t, filepath.Join(root, "song-b", "m2a.yml"), "score: song-b.mscz\n")

	projects, err := projectregistry.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2, got %d", len(projects))
	}
}

func TestScanSkipsInvalidMissingScore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bad", "m2a.yml"), "foo: bar\n")
	writeFile(t, filepath.Join(root, "good", "m2a.yml"), "score: good.mscz\n")

	projects, err := projectregistry.Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1, got %d", len(projects))
	}
	if projects[0].Name != "good" {
		t.Fatalf("expected good, got %s", projects[0].Name)
	}
}

func TestScanNonexistentRoot(t *testing.T) {
	projects, err := projectregistry.Scan([]string{`C:\does\not\exist\at\all`})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0, got %d", len(projects))
	}
}

func TestScanRelativeScorePath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "song-a", "m2a.yml"), "score: song-a.mscz\n")

	projects, _ := projectregistry.Scan([]string{root})
	expected := filepath.Join(root, "song-a", "song-a.mscz")
	if projects[0].ScorePath != expected {
		t.Fatalf("expected %s, got %s", expected, projects[0].ScorePath)
	}
}

func TestFindByScore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "song-a", "m2a.yml"), "score: song-a.mscz\n")

	projects, _ := projectregistry.Scan([]string{root})
	scorePath := filepath.Join(root, "song-a", "song-a.mscz")

	p := projectregistry.FindByScore(projects, scorePath)
	if p == nil {
		t.Fatal("expected match, got nil")
	}
	if p.Name != "song-a" {
		t.Fatalf("expected song-a, got %s", p.Name)
	}
}

func TestFindByScoreNoMatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "song-a", "m2a.yml"), "score: song-a.mscz\n")
	projects, _ := projectregistry.Scan([]string{root})

	p := projectregistry.FindByScore(projects, filepath.Join(root, "other", "file.mscz"))
	if p != nil {
		t.Fatalf("expected nil, got %v", p)
	}
}

func TestProjectFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "my-song", "m2a.yml"), "score: my-song.mscz\n")

	projects, _ := projectregistry.Scan([]string{root})
	if len(projects) != 1 {
		t.Fatalf("expected 1 project")
	}
	p := projects[0]
	if p.Name != "my-song" {
		t.Errorf("Name: expected my-song, got %s", p.Name)
	}
	if p.Dir != filepath.Join(root, "my-song") {
		t.Errorf("Dir: expected %s, got %s", filepath.Join(root, "my-song"), p.Dir)
	}
	if p.ConfigPath != filepath.Join(root, "my-song", "m2a.yml") {
		t.Errorf("ConfigPath: expected %s, got %s", filepath.Join(root, "my-song", "m2a.yml"), p.ConfigPath)
	}
}
