//go:build linux

package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/room215/limier/internal/collector"
)

const (
	ebpfIntegrationEnv   = "LIMIER_EBPF_INTEGRATION"
	ebpfIntegrationImage = "alpine:3.20"
	ebpfSmokeMarker      = "LIMIER_EBPF_SMOKE"
)

func TestManagerCapturesDockerExecWithEBPF(t *testing.T) {
	if os.Getenv(ebpfIntegrationEnv) != "1" {
		t.Skip("set LIMIER_EBPF_INTEGRATION=1 to run the Docker and eBPF integration test")
	}
	if os.Geteuid() != 0 {
		t.Fatal("Docker and eBPF integration test must run as root")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("locate docker: %v", err)
	}
	if _, err := exec.LookPath("bpftrace"); err != nil {
		t.Fatalf("locate bpftrace: %v", err)
	}
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		t.Fatalf("cgroup v2 is required: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if output, err := exec.CommandContext(ctx, "docker", "image", "inspect", ebpfIntegrationImage).CombinedOutput(); err != nil {
		t.Fatalf("inspect Docker image %q: %v\n%s", ebpfIntegrationImage, err, output)
	}

	runCollector, err := collector.NewFactory().Start(collector.RunContext{
		Side:     "candidate",
		RunIndex: 1,
	})
	if err != nil {
		t.Fatalf("start eBPF collector: %v", err)
	}

	result, err := NewManager("docker").Run(ctx, RunRequest{
		Side:        "candidate",
		RunIndex:    1,
		Image:       ebpfIntegrationImage,
		Workdir:     "/workspace",
		Workspace:   t.TempDir(),
		NetworkMode: "none",
		Steps: []Step{
			{
				Name:    "eBPF smoke",
				Intent:  "exercise",
				Command: "/bin/busybox echo " + ebpfSmokeMarker + " >/dev/null",
			},
		},
		EvidenceDir: t.TempDir(),
		Collector:   runCollector,
	})
	if err != nil {
		t.Fatalf("run Docker scenario with eBPF capture: %v", err)
	}

	for _, event := range result.Events {
		if event.Kind == "process.exec" &&
			event.Step == "eBPF smoke" &&
			strings.Contains(event.Command, ebpfSmokeMarker) {
			if result.EventsPath == "" {
				t.Fatal("EventsPath is empty after capturing an eBPF event")
			}
			if _, err := os.Stat(result.EventsPath); err != nil {
				t.Fatalf("stat persisted events %q: %v", result.EventsPath, err)
			}
			return
		}
	}

	t.Fatalf(
		"captured events = %#v, want process.exec for step %q containing %q; %s",
		result.Events,
		"eBPF smoke",
		ebpfSmokeMarker,
		diagnoseEBPFCgroup(ctx),
	)
}

func diagnoseEBPFCgroup(ctx context.Context) string {
	name := fmt.Sprintf("limier-ebpf-diagnostic-%d", time.Now().UnixNano())
	dockerBinary, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Sprintf("cgroup diagnostic failed to locate Docker: %v", err)
	}

	output, err := exec.CommandContext(
		ctx,
		dockerBinary,
		"run",
		"--detach",
		"--name",
		name,
		"--network",
		"none",
		ebpfIntegrationImage,
		"sh",
		"-c",
		`trap "exit 0" TERM INT; while true; do sleep 3600; done`,
	).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("cgroup diagnostic failed to start container: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}
	defer exec.CommandContext(context.Background(), dockerBinary, "rm", "-f", name).Run()

	cgroupPath, err := NewManager(dockerBinary).containerCgroupPath(ctx, name)
	if err != nil {
		return fmt.Sprintf("cgroup diagnostic failed to resolve container cgroup: %v", err)
	}

	script := fmt.Sprintf(`BEGIN
{
  printf("LIMIER_DIAG_TARGET\t%%llu\n", cgroupid(%q));
}

tracepoint:syscalls:sys_enter_execve,
tracepoint:syscalls:sys_enter_execveat
/str(args.filename) == "/bin/busybox"/
{
  printf("LIMIER_DIAG_ACTUAL\t%%llu\t%%d\t", cgroup, pid);
  join(args.argv, " ");
}
`, cgroupPath)

	childCommand := strings.Join([]string{
		dockerBinary,
		"exec",
		name,
		"/bin/busybox",
		"echo",
		ebpfSmokeMarker,
	}, " ")
	bpftrace := exec.CommandContext(ctx, "bpftrace", "-q", "-e", script, "-c", childCommand)
	bpftrace.Env = append(os.Environ(), "BPFTRACE_MAX_STRLEN=200")
	output, err = bpftrace.CombinedOutput()

	return fmt.Sprintf(
		"cgroup diagnostic: path=%q output=%q error=%v",
		cgroupPath,
		strings.TrimSpace(string(output)),
		err,
	)
}
