package rules

import "testing"

func TestValidateRejectsDuplicateRuleIDsInStableOrder(t *testing.T) {
	t.Parallel()

	file := File{
		Version: 1,
		HardBlock: []Rule{
			{ID: "dup", Finding: "step_failed"},
		},
		Review: []Rule{
			{ID: "dup", Finding: "step_stdout_changed"},
		},
		Suppress: []Rule{
			{ID: "dup", Finding: "step_stderr_changed"},
		},
	}

	want := "invalid rules: review[0].id duplicates hard_block[0]; suppress[0].id duplicates hard_block[0]"

	for range 50 {
		err := file.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want error")
		}
		if got := err.Error(); got != want {
			t.Fatalf("Validate() error = %q, want %q", got, want)
		}
	}
}

func TestRepositoryRuleFilesLoad(t *testing.T) {
	t.Parallel()

	paths := []string{
		"../../rules/default.yml",
		"../../rules/default-with-sample-noise.yml",
	}

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			file, err := Load(path)
			if err != nil {
				t.Fatalf("Load(%q) error = %v", path, err)
			}
			if file.Version != 1 {
				t.Fatalf("Load(%q).Version = %d, want 1", path, file.Version)
			}
		})
	}
}
