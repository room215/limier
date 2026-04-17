package cmd

import (
	"bytes"
	"testing"
)

func TestVersionCommandReportsLimierVersion(t *testing.T) {
	originalVersion := version
	version = "1.2.3"
	defer func() {
		version = originalVersion
	}()

	command := newVersionCommand()
	var output bytes.Buffer
	command.SetOut(&output)

	command.Run(command, nil)

	if got, want := output.String(), "limier 1.2.3\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}
