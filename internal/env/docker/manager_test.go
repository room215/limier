package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/room215/limier/internal/collector"
)

func TestOutputCaptureBoundsPreviewAndEvidence(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "step.stdout")
	capture, err := newOutputCapture(path, 5, 8)
	if err != nil {
		t.Fatalf("newOutputCapture() error = %v", err)
	}

	if _, err := capture.Write([]byte("hello world")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := capture.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	output := capture.Output()
	if output.Preview != "hello" {
		t.Fatalf("preview = %q, want %q", output.Preview, "hello")
	}
	if !output.Truncated {
		t.Fatal("output.Truncated = false, want true")
	}
	if output.TotalBytes != int64(len("hello world")) {
		t.Fatalf("output.TotalBytes = %d, want %d", output.TotalBytes, len("hello world"))
	}
	if output.SHA256 == "" {
		t.Fatal("output.SHA256 = empty, want digest")
	}

	file := capture.File()
	if file.StoredBytes != 8 {
		t.Fatalf("file.StoredBytes = %d, want 8", file.StoredBytes)
	}
	if !file.Truncated {
		t.Fatal("file.Truncated = false, want true")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); got != "hello wo" {
		t.Fatalf("evidence = %q, want %q", got, "hello wo")
	}
	if !strings.HasSuffix(file.Path, "step.stdout") {
		t.Fatalf("file.Path = %q, want suffix %q", file.Path, "step.stdout")
	}
}

func TestManagerLifecycleCommandsUseTimeouts(t *testing.T) {
	binary := writeFakeDocker(t, `exec sleep 1`)
	manager := Manager{
		binary:        binary,
		createTimeout: 25 * time.Millisecond,
		startTimeout:  25 * time.Millisecond,
		removeTimeout: 25 * time.Millisecond,
	}

	request := RunRequest{
		Image:       "alpine:3.20",
		Workdir:     "/workspace",
		Workspace:   t.TempDir(),
		EvidenceDir: t.TempDir(),
	}

	tests := []struct {
		name string
		run  func(context.Context) error
	}{
		{
			name: "create",
			run: func(ctx context.Context) error {
				return manager.createContainer(ctx, "limier-test", request)
			},
		},
		{
			name: "start",
			run: func(ctx context.Context) error {
				return manager.startContainer(ctx, "limier-test")
			},
		},
		{
			name: "rm",
			run: func(ctx context.Context) error {
				return manager.removeContainer(ctx, "limier-test")
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			start := time.Now()
			err := test.run(context.Background())
			if err == nil {
				t.Fatal("error = nil, want timeout")
			}
			if !strings.Contains(err.Error(), "timed out after") {
				t.Fatalf("error = %q, want timeout message", err)
			}
			if elapsed := time.Since(start); elapsed >= 500*time.Millisecond {
				t.Fatalf("elapsed = %s, want timeout before fake docker sleep finishes", elapsed)
			}
		})
	}
}

func TestRemoveContainerIgnoresMissingContainer(t *testing.T) {
	binary := writeFakeDocker(t, `printf "%s" "Error response from daemon: No such container: limier-missing\n" >&2`, `exit 1`)
	manager := Manager{
		binary:        binary,
		removeTimeout: 2 * time.Second,
	}

	if err := manager.removeContainer(context.Background(), "limier-missing"); err != nil {
		t.Fatalf("removeContainer() error = %v, want nil", err)
	}
}

func TestRunLifecycleCommandBoundsCapturedOutput(t *testing.T) {
	t.Parallel()

	binary := writeFakeDocker(t,
		`awk 'BEGIN { for (i = 0; i < 20000; i++) printf "x" }'`,
		`awk 'BEGIN { for (i = 0; i < 20000; i++) printf "y" > "/dev/stderr" }'`,
		`exit 1`,
	)
	manager := Manager{binary: binary}

	stdout, stderr, err := manager.runLifecycleCommand(context.Background(), time.Second, "create", "create")
	if err == nil {
		t.Fatal("runLifecycleCommand() error = nil, want lifecycle error")
	}
	if len(stdout) != lifecycleOutputLimitBytes {
		t.Fatalf("len(stdout) = %d, want %d", len(stdout), lifecycleOutputLimitBytes)
	}
	if len(stderr) != lifecycleOutputLimitBytes {
		t.Fatalf("len(stderr) = %d, want %d", len(stderr), lifecycleOutputLimitBytes)
	}
	if strings.Trim(stdout, "x") != "" {
		t.Fatalf("stdout = %q, want truncated x-only preview", stdout[:32])
	}
	if strings.Trim(stderr, "y") != "" {
		t.Fatalf("stderr = %q, want truncated y-only preview", stderr[:32])
	}
}

func TestCreateContainerRejectsMountsWithColon(t *testing.T) {
	t.Parallel()

	manager := Manager{binary: filepath.Join(t.TempDir(), "missing-docker")}
	request := RunRequest{
		Image:     "alpine:3.20",
		Workdir:   "/workspace",
		Workspace: t.TempDir(),
		Mounts: []Mount{
			{
				Source: "/tmp/source",
				Target: "/workspace:ro",
			},
		},
	}

	err := manager.createContainer(context.Background(), "limier-test", request)
	if err == nil {
		t.Fatal("createContainer() error = nil, want invalid mount error")
	}
	if !strings.Contains(err.Error(), `mount target "/workspace:ro" must not contain ':'`) {
		t.Fatalf("createContainer() error = %q, want invalid mount target message", err)
	}
}

func TestExecuteStepReturnsCollectorEventsWhenFinishFails(t *testing.T) {
	t.Parallel()

	binary := writeFakeDocker(t,
		`case "$1" in`,
		`  exec) /bin/sh -lc "$5"; exit $? ;;`,
		`  *) exit 0 ;;`,
		`esac`,
	)

	request := RunRequest{
		EvidenceDir: t.TempDir(),
		Collector: finishErrorRunCollector{
			err: errors.New("finish failed"),
			events: []collector.Event{
				{
					Kind:    "process.exec",
					Command: "child-helper --ping",
				},
			},
		},
	}

	result, events, err := Manager{binary: binary}.executeStep(
		context.Background(),
		"limier-test",
		"/sys/fs/cgroup/fake",
		request,
		0,
		Step{Name: "exercise", Intent: "exercise", Command: "printf ok"},
	)
	if err == nil {
		t.Fatal("executeStep() error = nil, want collector finish error")
	}
	if !strings.Contains(err.Error(), "finish failed") {
		t.Fatalf("executeStep() error = %q, want collector finish error", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Command != "child-helper --ping" {
		t.Fatalf("event command = %q, want %q", events[0].Command, "child-helper --ping")
	}
	if result.Evidence.Stdout.Path == "" || result.Evidence.Stderr.Path == "" {
		t.Fatalf("result evidence = %#v, want stdout and stderr paths", result.Evidence)
	}
}

func TestPersistEventsWritesEventsEvidence(t *testing.T) {
	t.Parallel()

	result := RunResult{
		Events: []collector.Event{
			{
				Kind:      "process.exec",
				Step:      "exercise",
				Command:   "child-helper --ping",
				Timestamp: time.Unix(0, 1).UTC(),
			},
		},
	}

	evidenceDir := t.TempDir()
	if err := persistEvents(&result, evidenceDir); err != nil {
		t.Fatalf("persistEvents() error = %v", err)
	}
	if result.EventsPath == "" {
		t.Fatal("EventsPath = empty, want persisted events evidence")
	}
	if _, err := os.Stat(result.EventsPath); err != nil {
		t.Fatalf("Stat(%q) error = %v, want persisted events file", result.EventsPath, err)
	}
	if len(result.EvidenceFiles) != 1 || result.EvidenceFiles[0] != result.EventsPath {
		t.Fatalf("evidence files = %#v, want events path only", result.EvidenceFiles)
	}
}

type finishErrorRunCollector struct {
	err    error
	events []collector.Event
}

func (c finishErrorRunCollector) StartStepCapture(_ context.Context, step collector.StepContext) (collector.StepCapture, error) {
	events := make([]collector.Event, 0, len(c.events))
	for _, event := range c.events {
		event.Step = step.Name
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Unix(0, 1).UTC()
		}
		events = append(events, event)
	}

	return finishErrorStepCapture{
		err:    c.err,
		events: events,
	}, nil
}

type finishErrorStepCapture struct {
	err    error
	events []collector.Event
}

func (c finishErrorStepCapture) Finish(context.Context) ([]collector.Event, error) {
	return c.events, c.err
}

func writeFakeDocker(t *testing.T, commands ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "docker")
	lines := append([]string{"#!/bin/sh"}, commands...)
	script := strings.Join(append(lines, ""), "\n")

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
