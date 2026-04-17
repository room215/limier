package adapter

import (
	"context"

	"github.com/oneslash/limier/internal/scenario"
)

type DependencySpec struct {
	Package string
	Version string
}

type Metadata struct {
	ManifestPath          string
	LockfilePath          string
	InstalledVersionPath  string
	InstalledVersionKind  string
	AdditionalReportPairs map[string]string
}

type PreparedWorkspace struct {
	InstallCommand string
	Metadata       Metadata
}

type StepPlan struct {
	Name    string
	Intent  string
	Command string
}

type Adapter interface {
	Ecosystem() string
	DefaultImage() string
	PrepareWorkspace(context.Context, string, DependencySpec) (PreparedWorkspace, error)
	ResolveStep(scenario.Step, PreparedWorkspace) (StepPlan, error)
	ReadInstalledVersion(string, DependencySpec, PreparedWorkspace) (string, error)
}

func (p PreparedWorkspace) ReportMetadata() map[string]string {
	metadata := map[string]string{}

	if p.InstallCommand != "" {
		metadata["install_command"] = p.InstallCommand
	}

	if p.Metadata.ManifestPath != "" {
		metadata["manifest_path"] = p.Metadata.ManifestPath
	}

	if p.Metadata.LockfilePath != "" {
		metadata["lockfile_path"] = p.Metadata.LockfilePath
	}

	if p.Metadata.InstalledVersionPath != "" {
		metadata["installed_version_path"] = p.Metadata.InstalledVersionPath
	}

	if p.Metadata.InstalledVersionKind != "" {
		metadata["installed_version_kind"] = p.Metadata.InstalledVersionKind
	}

	for key, value := range p.Metadata.AdditionalReportPairs {
		if key == "" || value == "" {
			continue
		}
		metadata[key] = value
	}

	if len(metadata) == 0 {
		return nil
	}

	return metadata
}
