package preset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/room215/limier/internal/scenario"
)

func TestResolveScenarioSupportsDefaultEcosystemPresets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		ecosystem string
		wantName  string
		wantSteps string
	}{
		{
			ecosystem: "cargo",
			wantName:  "cargo generic dependency review",
			wantSteps: "install dependency",
		},
		{
			ecosystem: "npm",
			wantName:  "npm generic dependency review",
			wantSteps: "install dependency,exercise package",
		},
		{
			ecosystem: "pip",
			wantName:  "pip generic dependency review",
			wantSteps: "install dependency,probe import",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.ecosystem, func(t *testing.T) {
			t.Parallel()

			path, cleanup, err := ResolveScenario("", testCase.ecosystem)
			if err != nil {
				t.Fatalf("ResolveScenario() error = %v", err)
			}
			t.Cleanup(cleanup)

			manifest, err := scenario.Load(path)
			if err != nil {
				t.Fatalf("scenario.Load() error = %v", err)
			}

			if manifest.Name != testCase.wantName {
				t.Fatalf("manifest.Name = %q, want %q", manifest.Name, testCase.wantName)
			}
			if got := strings.Join(manifest.StepNames(), ","); got != testCase.wantSteps {
				t.Fatalf("manifest.StepNames() = %q, want %q", got, testCase.wantSteps)
			}
		})
	}
}

func TestResolveFixtureGeneratesPipRequirementForPackage(t *testing.T) {
	t.Parallel()

	path, cleanup, err := ResolveFixture("", "pip", "requests")
	if err != nil {
		t.Fatalf("ResolveFixture() error = %v", err)
	}
	t.Cleanup(cleanup)

	data, err := os.ReadFile(filepath.Join(path, "requirements.txt"))
	if err != nil {
		t.Fatalf("ReadFile(requirements.txt) error = %v", err)
	}

	if got := string(data); got != "requests==0.0.0\n" {
		t.Fatalf("requirements.txt = %q, want package placeholder", got)
	}
}

func TestResolveFixtureGeneratesCargoCrateForPackage(t *testing.T) {
	t.Parallel()

	path, cleanup, err := ResolveFixture("", "cargo", "serde-json")
	if err != nil {
		t.Fatalf("ResolveFixture() error = %v", err)
	}
	t.Cleanup(cleanup)

	data, err := os.ReadFile(filepath.Join(path, "Cargo.toml"))
	if err != nil {
		t.Fatalf("ReadFile(Cargo.toml) error = %v", err)
	}
	if !strings.Contains(string(data), `"serde-json" = "0.0.0"`) {
		t.Fatalf("Cargo.toml = %q, want quoted dependency placeholder", string(data))
	}

	if _, err := os.Stat(filepath.Join(path, "src", "lib.rs")); err != nil {
		t.Fatalf("Stat(src/lib.rs) error = %v", err)
	}
}

func TestResolveFixtureRejectsUnsafeGeneratedPackageNames(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		ecosystem   string
		packageName string
	}{
		{
			name:        "pip missing",
			ecosystem:   "pip",
			packageName: "",
		},
		{
			name:        "pip newline",
			ecosystem:   "pip",
			packageName: "requests\nurllib3",
		},
		{
			name:        "cargo semicolon",
			ecosystem:   "cargo",
			packageName: "serde;rm",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path, cleanup, err := ResolveFixture("", testCase.ecosystem, testCase.packageName)
			if cleanup != nil {
				t.Cleanup(cleanup)
			}
			if err == nil {
				t.Fatalf("ResolveFixture() error = nil, want error (path %q)", path)
			}
		})
	}
}
