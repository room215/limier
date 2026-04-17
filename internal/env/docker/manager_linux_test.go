//go:build linux

package docker

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/oneslash/limier/internal/collector"
)

func TestManagerRunUsesStepScopedCollectorEvents(t *testing.T) {
	t.Parallel()

	testPID := os.Getpid()
	binary := writeFakeDocker(t,
		`case "$1" in`,
		`  create|start|rm) exit 0 ;;`,
		`  inspect) printf "%d\n" `+strconv.Itoa(testPID)+`; exit 0 ;;`,
		`  exec) /bin/sh -lc "$5"; exit $? ;;`,
		`  *) exit 1 ;;`,
		`esac`,
	)

	runCollector := &fakeRunCollector{
		eventsByStep: map[string][]collector.Event{
			"exercise": {
				{
					Kind:    "process.exec",
					Command: "child-helper --ping",
				},
			},
		},
	}

	result, err := Manager{binary: binary}.Run(context.Background(), RunRequest{
		Side:      "candidate",
		RunIndex:  1,
		Image:     "alpine:3.20",
		Workdir:   "/workspace",
		Workspace: t.TempDir(),
		Steps: []Step{
			{Name: "exercise", Intent: "exercise", Command: "echo ok"},
		},
		EvidenceDir: t.TempDir(),
		Collector:   runCollector,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(runCollector.startedSteps) != 1 {
		t.Fatalf("len(startedSteps) = %d, want 1", len(runCollector.startedSteps))
	}
	if got := runCollector.startedSteps[0].ContainerCgroupPath; got == "" {
		t.Fatal("ContainerCgroupPath = empty, want resolved cgroup path")
	}
	if len(result.Events) != 1 {
		t.Fatalf("len(result.Events) = %d, want 1", len(result.Events))
	}
	if got := result.Events[0].Command; got != "child-helper --ping" {
		t.Fatalf("event command = %q, want %q", got, "child-helper --ping")
	}
	if got := result.Events[0].Step; got != "exercise" {
		t.Fatalf("event step = %q, want %q", got, "exercise")
	}
	if result.Events[0].Command == "echo ok" {
		t.Fatalf("result.Events = %#v, want no synthetic wrapper event", result.Events)
	}
	if result.EventsPath == "" {
		t.Fatal("EventsPath = empty, want evidence file path")
	}
}

type fakeRunCollector struct {
	startedSteps []collector.StepContext
	eventsByStep map[string][]collector.Event
}

func (c *fakeRunCollector) StartStepCapture(_ context.Context, step collector.StepContext) (collector.StepCapture, error) {
	c.startedSteps = append(c.startedSteps, step)

	events := make([]collector.Event, 0, len(c.eventsByStep[step.Name]))
	for _, event := range c.eventsByStep[step.Name] {
		event.Step = step.Name
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Unix(0, 1).UTC()
		}
		events = append(events, event)
	}

	return fakeStepCapture{events: events}, nil
}

type fakeStepCapture struct {
	events []collector.Event
}

func (c fakeStepCapture) Finish(context.Context) ([]collector.Event, error) {
	return c.events, nil
}
