package projectregistry

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Project represents a discovered m2a project under a root directory.
type Project struct {
	Dir        string // absolute path to the folder containing m2a.yml
	Name       string // base name of Dir
	ScorePath  string // absolute path to the .mscz file
	ConfigPath string // absolute path to m2a.yml
}

type projectYML struct {
	Score string `yaml:"score"`
}

// Scan walks each root recursively and collects all valid projects.
// Non-existent roots are skipped with a log warning (not an error).
func Scan(roots []string) ([]Project, error) {
	var projects []Project
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			log.Printf("projectregistry: root %s does not exist, skipping", root)
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || d.Name() != "m2a.yml" {
				return nil
			}
			p, parseErr := parseProject(path)
			if parseErr != nil {
				log.Printf("projectregistry: skipping %s: %v", path, parseErr)
				return nil
			}
			projects = append(projects, p)
			return nil
		})
		if err != nil {
			log.Printf("projectregistry: error scanning %s: %v", root, err)
		}
	}
	return projects, nil
}

func parseProject(configPath string) (Project, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Project{}, err
	}
	var yml projectYML
	if err := yaml.Unmarshal(data, &yml); err != nil {
		return Project{}, err
	}
	if yml.Score == "" {
		return Project{}, fmt.Errorf("missing score field")
	}
	dir := filepath.Dir(configPath)
	scorePath := yml.Score
	if !filepath.IsAbs(scorePath) {
		scorePath = filepath.Join(dir, scorePath)
	}
	return Project{
		Dir:        dir,
		Name:       filepath.Base(dir),
		ScorePath:  filepath.Clean(scorePath),
		ConfigPath: configPath,
	}, nil
}

// FindByScore returns the project whose ScorePath matches scorePath, or nil.
func FindByScore(projects []Project, scorePath string) *Project {
	clean := filepath.Clean(scorePath)
	for i := range projects {
		if projects[i].ScorePath == clean {
			return &projects[i]
		}
	}
	return nil
}
