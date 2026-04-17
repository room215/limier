package scenario

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultRepeats  = 2
	defaultWorkdir  = "/workspace"
	defaultExitCode = 0
)

type Manifest struct {
	Version  int               `yaml:"version"`
	Name     string            `yaml:"name"`
	Repeats  int               `yaml:"repeats"`
	Image    string            `yaml:"image,omitempty"`
	Workdir  string            `yaml:"workdir,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Steps    []Step            `yaml:"steps"`
	Success  Success           `yaml:"success,omitempty"`
	Network  Network           `yaml:"network,omitempty"`
	Mounts   []Mount           `yaml:"mounts,omitempty"`
	Evidence Evidence          `yaml:"evidence,omitempty"`
}

type Step struct {
	Name    string `yaml:"name"`
	Run     string `yaml:"run"`
	Command string `yaml:"command,omitempty"`
}

type Success struct {
	ExitCode int `yaml:"exit_code"`
}

type Network struct {
	Mode string `yaml:"mode,omitempty"`
}

type Mount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only,omitempty"`
}

type Evidence struct {
	CaptureHostSignals *bool `yaml:"capture_host_signals,omitempty"`
}

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read scenario %q: %w", path, err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse scenario %q: %w", path, err)
	}

	manifest.applyDefaults()
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func (m *Manifest) applyDefaults() {
	if m.Repeats == 0 {
		m.Repeats = defaultRepeats
	}

	if strings.TrimSpace(m.Workdir) == "" {
		m.Workdir = defaultWorkdir
	}

	if m.Success.ExitCode == 0 {
		m.Success.ExitCode = defaultExitCode
	}

	if m.Env == nil {
		m.Env = map[string]string{}
	}

	if strings.TrimSpace(m.Network.Mode) == "" {
		m.Network.Mode = "default"
	}

}

func (m Manifest) Validate() error {
	var problems []string

	if m.Version == 0 {
		problems = append(problems, "version is required")
	} else if m.Version != 1 {
		problems = append(problems, "version must be 1")
	}

	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, "name is required")
	}

	if m.Repeats <= 0 {
		problems = append(problems, "repeats must be greater than 0")
	}

	if strings.TrimSpace(m.Workdir) == "" {
		problems = append(problems, "workdir is required")
	}

	if len(m.Steps) == 0 {
		problems = append(problems, "at least one step is required")
	}

	installStepCount := 0
	for index, step := range m.Steps {
		prefix := fmt.Sprintf("steps[%d]", index)
		if strings.TrimSpace(step.Name) == "" {
			problems = append(problems, prefix+".name is required")
		}

		if strings.TrimSpace(step.Run) == "" {
			problems = append(problems, prefix+".run is required")
			continue
		}

		if step.Run == "install" {
			installStepCount++
			if strings.TrimSpace(step.Command) != "" {
				problems = append(problems, prefix+".command must be empty for install steps")
			}
			continue
		}

		if strings.TrimSpace(step.Command) == "" {
			problems = append(problems, prefix+".command is required for non-install steps")
		}
	}

	if installStepCount == 0 {
		problems = append(problems, "at least one install step is required")
	}

	switch m.Network.Mode {
	case "default", "none":
	default:
		problems = append(problems, "network.mode must be default or none")
	}

	for index, mount := range m.Mounts {
		prefix := fmt.Sprintf("mounts[%d]", index)
		if strings.TrimSpace(mount.Source) == "" {
			problems = append(problems, prefix+".source is required")
		}
		if strings.TrimSpace(mount.Target) == "" {
			problems = append(problems, prefix+".target is required")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid scenario: %s", strings.Join(problems, "; "))
	}

	return nil
}

func (m Manifest) StepNames() []string {
	names := make([]string, 0, len(m.Steps))
	for _, step := range m.Steps {
		names = append(names, step.Name)
	}

	return names
}

func (e Evidence) HostSignalsEnabled() bool {
	if e.CaptureHostSignals == nil {
		return true
	}

	return *e.CaptureHostSignals
}
