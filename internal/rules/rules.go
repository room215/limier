package rules

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type File struct {
	Version   int    `yaml:"version"`
	HardBlock []Rule `yaml:"hard_block,omitempty"`
	Review    []Rule `yaml:"review,omitempty"`
	Suppress  []Rule `yaml:"suppress,omitempty"`
}

type Rule struct {
	ID              string `yaml:"id"`
	Finding         string `yaml:"finding"`
	Step            string `yaml:"step,omitempty"`
	MessageContains string `yaml:"message_contains,omitempty"`
	Reason          string `yaml:"reason,omitempty"`
}

func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read rules %q: %w", path, err)
	}

	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("parse rules %q: %w", path, err)
	}

	if err := file.Validate(); err != nil {
		return File{}, err
	}

	return file, nil
}

func (f File) Validate() error {
	var problems []string

	if f.Version == 0 {
		problems = append(problems, "version is required")
	} else if f.Version != 1 {
		problems = append(problems, "version must be 1")
	}

	seen := map[string]string{}
	for _, ruleset := range []struct {
		category string
		rules    []Rule
	}{
		{category: "hard_block", rules: f.HardBlock},
		{category: "review", rules: f.Review},
		{category: "suppress", rules: f.Suppress},
	} {
		for index, rule := range ruleset.rules {
			prefix := fmt.Sprintf("%s[%d]", ruleset.category, index)
			if strings.TrimSpace(rule.ID) == "" {
				problems = append(problems, prefix+".id is required")
			}

			if strings.TrimSpace(rule.Finding) == "" {
				problems = append(problems, prefix+".finding is required")
			}

			if previous, ok := seen[rule.ID]; ok {
				problems = append(problems, fmt.Sprintf("%s.id duplicates %s", prefix, previous))
			} else if strings.TrimSpace(rule.ID) != "" {
				seen[rule.ID] = prefix
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid rules: %s", strings.Join(problems, "; "))
	}

	return nil
}
