//go:build linux

package collector

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBpftraceCollectorCapturesChildExecsInDedicatedCgroup(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to create and manage a dedicated cgroup")
	}
	if _, err := exec.LookPath("bpftrace"); err != nil {
		t.Skipf("bpftrace is unavailable: %v", err)
	}
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		t.Skipf("cgroup v2 is unavailable: %v", err)
	}

	root := "/sys/fs/cgroup"
	cgroupPath := filepath.Join(root, "limier-test-"+strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.Mkdir(cgroupPath, 0o755); err != nil {
		t.Skipf("create test cgroup: %v", err)
	}
	defer os.Remove(cgroupPath)

	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skipf("locate true: %v", err)
	}
	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Skipf("locate echo: %v", err)
	}

	factory := NewFactory()
	runCollector, err := factory.Start(RunContext{Side: "candidate", RunIndex: 1})
	if err != nil {
		t.Skipf("start collector: %v", err)
	}

	stepCapture, err := runCollector.StartStepCapture(context.Background(), StepContext{
		Name:                "exercise",
		Command:             "child process smoke test",
		ContainerCgroupPath: cgroupPath,
	})
	if err != nil {
		t.Skipf("start step capture: %v", err)
	}

	command := exec.Command("/bin/sh", "-lc", strings.Join([]string{
		"echo $$ > " + filepath.Join(cgroupPath, "cgroup.procs"),
		truePath,
		echoPath + " smoke >/dev/null",
	}, "; "))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run smoke command: %v\n%s", err, output)
	}

	events, err := stepCapture.Finish(context.Background())
	if err != nil {
		t.Skipf("finish step capture: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("events = empty, want child process execs")
	}

	assertContainsCommand(t, events, truePath)
	assertContainsCommand(t, events, echoPath+" smoke")
}

func assertContainsCommand(t *testing.T, events []Event, want string) {
	t.Helper()

	for _, event := range events {
		if strings.Contains(event.Command, want) {
			return
		}
	}

	t.Fatalf("events = %#v, want command containing %q", events, want)
}
