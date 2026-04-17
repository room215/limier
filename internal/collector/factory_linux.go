//go:build linux

package collector

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	bpftraceReadyMarker   = "LIMIER_READY"
	bpftraceEventPrefix   = "LIMIER_EVENT\t"
	bpftraceStartTimeout  = 10 * time.Second
	bpftraceMaxStringSize = "4096"
)

type bpftraceFactory struct{}

type bpftraceRunCollector struct {
	binary string
	run    RunContext
}

type bpftraceStepCapture struct {
	stepName   string
	scriptPath string
	cmd        *exec.Cmd
	stderr     bytes.Buffer
	waitDone   chan struct{}
	readyCh    chan struct{}

	mu         sync.Mutex
	events     []Event
	stdoutErr  error
	waitErr    error
	readyOnce  sync.Once
	stopSignal bool
}

func newFactory() Factory {
	return bpftraceFactory{}
}

func (bpftraceFactory) Start(run RunContext) (RunCollector, error) {
	binary, err := exec.LookPath("bpftrace")
	if err != nil {
		return nil, &CaptureError{
			Op:  "start host signal collector",
			Err: fmt.Errorf("locate bpftrace: %w", err),
		}
	}

	return &bpftraceRunCollector{
		binary: binary,
		run:    run,
	}, nil
}

func (c *bpftraceRunCollector) StartStepCapture(ctx context.Context, step StepContext) (StepCapture, error) {
	if strings.TrimSpace(step.ContainerCgroupPath) == "" {
		return nil, &CaptureError{
			Op:   "start step capture",
			Step: step.Name,
			Err:  errors.New("container cgroup path is required"),
		}
	}

	scriptFile, err := os.CreateTemp("", "limier-bpftrace-*.bt")
	if err != nil {
		return nil, &CaptureError{
			Op:   "start step capture",
			Step: step.Name,
			Err:  fmt.Errorf("create bpftrace script: %w", err),
		}
	}

	scriptPath := scriptFile.Name()
	if _, err := scriptFile.WriteString(buildBpftraceScript(step.ContainerCgroupPath)); err != nil {
		_ = scriptFile.Close()
		_ = os.Remove(scriptPath)
		return nil, &CaptureError{
			Op:   "start step capture",
			Step: step.Name,
			Err:  fmt.Errorf("write bpftrace script %q: %w", scriptPath, err),
		}
	}

	if err := scriptFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return nil, &CaptureError{
			Op:   "start step capture",
			Step: step.Name,
			Err:  fmt.Errorf("close bpftrace script %q: %w", scriptPath, err),
		}
	}

	cmd := exec.Command(c.binary, "-q", scriptPath)
	cmd.Env = append(os.Environ(), "BPFTRACE_MAX_STRLEN="+bpftraceMaxStringSize)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = os.Remove(scriptPath)
		return nil, &CaptureError{
			Op:   "start step capture",
			Step: step.Name,
			Err:  fmt.Errorf("open bpftrace stdout: %w", err),
		}
	}

	stepCapture := &bpftraceStepCapture{
		stepName:   step.Name,
		scriptPath: scriptPath,
		cmd:        cmd,
		waitDone:   make(chan struct{}),
		readyCh:    make(chan struct{}),
	}
	cmd.Stderr = &stepCapture.stderr

	stepCapture.readStdout(stdout)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return nil, &CaptureError{
			Op:   "start step capture",
			Step: step.Name,
			Err:  fmt.Errorf("start bpftrace: %w", err),
		}
	}

	go func() {
		stepCapture.mu.Lock()
		stepCapture.waitErr = cmd.Wait()
		stepCapture.mu.Unlock()
		close(stepCapture.waitDone)
	}()

	startCtx, cancel := context.WithTimeout(ctx, bpftraceStartTimeout)
	defer cancel()

	if err := stepCapture.waitUntilReady(startCtx); err != nil {
		_ = stepCapture.stop(context.Background())
		_ = os.Remove(scriptPath)
		return nil, &CaptureError{
			Op:   "start step capture",
			Step: step.Name,
			Err:  err,
		}
	}

	return stepCapture, nil
}

func (c *bpftraceStepCapture) Finish(ctx context.Context) ([]Event, error) {
	defer func() {
		_ = os.Remove(c.scriptPath)
	}()

	if err := c.stop(ctx); err != nil {
		return nil, &CaptureError{
			Op:   "finish step capture",
			Step: c.stepName,
			Err:  err,
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	events := make([]Event, len(c.events))
	copy(events, c.events)

	if c.stdoutErr != nil {
		return events, &CaptureError{
			Op:   "finish step capture",
			Step: c.stepName,
			Err:  c.stdoutErr,
		}
	}

	return events, nil
}

func (c *bpftraceStepCapture) waitUntilReady(ctx context.Context) error {
	select {
	case <-c.readyCh:
		return nil
	case <-c.waitDone:
		err := c.commandWaitErr()
		if err == nil {
			return errors.New("bpftrace exited before reporting readiness")
		}
		return fmt.Errorf("bpftrace exited before reporting readiness: %w; stderr: %s", err, strings.TrimSpace(c.stderr.String()))
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for bpftrace readiness: %w; stderr: %s", ctx.Err(), strings.TrimSpace(c.stderr.String()))
	}
}

func (c *bpftraceStepCapture) stop(ctx context.Context) error {
	if c.cmd.Process == nil {
		return nil
	}

	c.stopSignal = true
	if err := c.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("signal bpftrace: %w", err)
	}

	select {
	case <-c.waitDone:
		err := c.commandWaitErr()
		if err == nil {
			return nil
		}
		if c.stopSignal && isInterruptExit(err) {
			return nil
		}
		return fmt.Errorf("wait for bpftrace: %w; stderr: %s", err, strings.TrimSpace(c.stderr.String()))
	case <-ctx.Done():
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		<-c.waitDone
		err := c.commandWaitErr()
		if err == nil {
			return nil
		}
		return fmt.Errorf("wait for bpftrace after timeout: %w; stderr: %s", ctx.Err(), strings.TrimSpace(c.stderr.String()))
	}
}

func (c *bpftraceStepCapture) commandWaitErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.waitErr
}

func (c *bpftraceStepCapture) readStdout(stdout io.ReadCloser) {
	go func() {
		defer stdout.Close()

		scanner := bufio.NewScanner(stdout)
		buffer := make([]byte, 0, 128*1024)
		scanner.Buffer(buffer, 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == bpftraceReadyMarker:
				c.readyOnce.Do(func() {
					close(c.readyCh)
				})
			case strings.HasPrefix(line, bpftraceEventPrefix):
				event, err := parseBpftraceEvent(strings.TrimPrefix(line, bpftraceEventPrefix))
				if err != nil {
					c.mu.Lock()
					if c.stdoutErr == nil {
						c.stdoutErr = err
					}
					c.mu.Unlock()
					continue
				}
				event.Step = c.stepName

				c.mu.Lock()
				c.events = append(c.events, event)
				c.mu.Unlock()
			}
		}

		if err := scanner.Err(); err != nil {
			c.mu.Lock()
			if c.stdoutErr == nil {
				c.stdoutErr = fmt.Errorf("read bpftrace stdout: %w", err)
			}
			c.mu.Unlock()
		}
	}()
}

func parseBpftraceEvent(line string) (Event, error) {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return Event{}, fmt.Errorf("parse bpftrace event %q: expected timestamp and command", line)
	}

	nsecs, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return Event{}, fmt.Errorf("parse bpftrace timestamp %q: %w", parts[0], err)
	}

	return Event{
		Kind:      "process.exec",
		Command:   strings.TrimSpace(parts[1]),
		Timestamp: time.Unix(0, nsecs).UTC(),
	}, nil
}

func buildBpftraceScript(cgroupPath string) string {
	return fmt.Sprintf(`BEGIN
{
  printf("%s\n");
}

tracepoint:syscalls:sys_enter_execve,
tracepoint:syscalls:sys_enter_execveat
/cgroup == cgroupid(%q)/
{
  printf("%s%%llu\t", nsecs);
  join(args.argv, " ");
}
`, bpftraceReadyMarker, cgroupPath, bpftraceEventPrefix)
}

func isInterruptExit(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}

	return waitStatus.Signaled() && waitStatus.Signal() == syscall.SIGINT
}
