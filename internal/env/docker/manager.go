package docker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/room215/limier/internal/collector"
)

const (
	outputPreviewLimitBytes   = 64 * 1024
	outputEvidenceLimitBytes  = 1024 * 1024
	lifecycleOutputLimitBytes = 16 * 1024
	defaultCreateTimeout      = 2 * time.Minute
	defaultStartTimeout       = 30 * time.Second
	defaultRemoveTimeout      = 15 * time.Second
	defaultInspectTimeout     = 10 * time.Second
	defaultCollectorStopGrace = 5 * time.Second
)

type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type Step struct {
	Name    string
	Intent  string
	Command string
}

type StepEvidence struct {
	Stdout OutputFile `json:"stdout,omitempty"`
	Stderr OutputFile `json:"stderr,omitempty"`
}

type Output struct {
	Preview    string `json:"preview,omitempty"`
	TotalBytes int64  `json:"total_bytes"`
	SHA256     string `json:"sha256"`
	Truncated  bool   `json:"truncated,omitempty"`
}

type OutputFile struct {
	Path        string `json:"path,omitempty"`
	StoredBytes int64  `json:"stored_bytes"`
	Truncated   bool   `json:"truncated,omitempty"`
}

type StepResult struct {
	Name                 string       `json:"name"`
	Intent               string       `json:"intent"`
	Command              string       `json:"command"`
	ExitCode             int          `json:"exit_code"`
	DurationMilliseconds int64        `json:"duration_ms"`
	Stdout               Output       `json:"stdout"`
	Stderr               Output       `json:"stderr"`
	Evidence             StepEvidence `json:"evidence"`
}

type RunRequest struct {
	Side        string
	RunIndex    int
	Image       string
	Workdir     string
	Workspace   string
	Env         map[string]string
	Mounts      []Mount
	NetworkMode string
	Steps       []Step
	EvidenceDir string
	Collector   collector.RunCollector
}

type RunResult struct {
	RunIndex             int               `json:"run_index"`
	ExitCode             int               `json:"exit_code"`
	DurationMilliseconds int64             `json:"duration_ms"`
	Steps                []StepResult      `json:"steps"`
	Events               []collector.Event `json:"events,omitempty"`
	EventsPath           string            `json:"events_path,omitempty"`
	EvidenceFiles        []string          `json:"evidence_files,omitempty"`
}

type Manager struct {
	binary        string
	createTimeout time.Duration
	startTimeout  time.Duration
	removeTimeout time.Duration
}

func NewManager(binary string) Manager {
	if strings.TrimSpace(binary) == "" {
		binary = "docker"
	}

	return Manager{
		binary:        binary,
		createTimeout: defaultCreateTimeout,
		startTimeout:  defaultStartTimeout,
		removeTimeout: defaultRemoveTimeout,
	}
}

func (m Manager) Run(ctx context.Context, request RunRequest) (RunResult, error) {
	containerName := fmt.Sprintf("limier-%s-%d-%d", request.Side, request.RunIndex, time.Now().UnixNano())
	result := RunResult{
		RunIndex: request.RunIndex,
	}

	if err := os.MkdirAll(request.EvidenceDir, 0o755); err != nil {
		return result, fmt.Errorf("create evidence directory %q: %w", request.EvidenceDir, err)
	}

	if err := m.createContainer(ctx, containerName, request); err != nil {
		return result, err
	}
	defer func() {
		_ = m.removeContainer(context.Background(), containerName)
	}()

	if err := m.startContainer(ctx, containerName); err != nil {
		return result, err
	}

	containerCgroupPath := ""
	if request.Collector != nil {
		path, err := m.containerCgroupPath(ctx, containerName)
		if err != nil {
			return result, &collector.CaptureError{
				Op:  "resolve container cgroup",
				Err: err,
			}
		}
		containerCgroupPath = path
	}

	runStart := time.Now()
	for index, step := range request.Steps {
		stepResult, stepEvents, err := m.executeStep(ctx, containerName, containerCgroupPath, request, index, step)
		result.Steps = append(result.Steps, stepResult)
		result.EvidenceFiles = append(result.EvidenceFiles, stepResult.Evidence.Stdout.Path, stepResult.Evidence.Stderr.Path)
		result.Events = append(result.Events, stepEvents...)
		if err != nil {
			if writeErr := persistEvents(&result, request.EvidenceDir); writeErr != nil {
				return result, writeErr
			}
			return result, err
		}

		result.ExitCode = stepResult.ExitCode
		if stepResult.ExitCode != 0 {
			break
		}
	}

	result.DurationMilliseconds = time.Since(runStart).Milliseconds()
	if err := persistEvents(&result, request.EvidenceDir); err != nil {
		return result, err
	}

	return result, nil
}

func (m Manager) createContainer(ctx context.Context, name string, request RunRequest) error {
	args := []string{
		"create",
		"--name", name,
		"--workdir", request.Workdir,
		"--volume", fmt.Sprintf("%s:%s", request.Workspace, request.Workdir),
	}

	envKeys := make([]string, 0, len(request.Env))
	for key := range request.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		args = append(args, "--env", key+"="+request.Env[key])
	}

	for _, mount := range request.Mounts {
		if err := validateMountPath("source", mount.Source); err != nil {
			return err
		}
		if err := validateMountPath("target", mount.Target); err != nil {
			return err
		}

		spec := mount.Source + ":" + mount.Target
		if mount.ReadOnly {
			spec += ":ro"
		}
		args = append(args, "--volume", spec)
	}

	if request.NetworkMode == "none" {
		args = append(args, "--network", "none")
	}

	args = append(args, request.Image, "sh", "-lc", `trap "exit 0" TERM INT; while true; do sleep 3600; done`)

	_, _, err := m.runLifecycleCommand(ctx, m.lifecycleTimeout(m.createTimeout, defaultCreateTimeout), "create", args...)
	return err
}

func (m Manager) startContainer(ctx context.Context, name string) error {
	_, _, err := m.runLifecycleCommand(ctx, m.lifecycleTimeout(m.startTimeout, defaultStartTimeout), "start", "start", name)
	return err
}

func (m Manager) removeContainer(ctx context.Context, name string) error {
	_, stderr, err := m.runLifecycleCommand(ctx, m.lifecycleTimeout(m.removeTimeout, defaultRemoveTimeout), "rm", "rm", "-f", name)
	if err != nil {
		if strings.Contains(stderr, "No such container") {
			return nil
		}
		return err
	}

	return nil
}

func (m Manager) executeStep(ctx context.Context, containerName string, containerCgroupPath string, request RunRequest, index int, step Step) (StepResult, []collector.Event, error) {
	safeName := sanitizeFilename(step.Name)
	stdoutPath := filepath.Join(request.EvidenceDir, fmt.Sprintf("%02d-%s.stdout", index+1, safeName))
	stderrPath := filepath.Join(request.EvidenceDir, fmt.Sprintf("%02d-%s.stderr", index+1, safeName))

	stdoutCapture, err := newOutputCapture(stdoutPath, outputPreviewLimitBytes, outputEvidenceLimitBytes)
	if err != nil {
		return StepResult{}, nil, err
	}

	stderrCapture, err := newOutputCapture(stderrPath, outputPreviewLimitBytes, outputEvidenceLimitBytes)
	if err != nil {
		_ = stdoutCapture.Close()
		return StepResult{}, nil, err
	}

	var stepCapture collector.StepCapture
	if request.Collector != nil {
		stepCapture, err = request.Collector.StartStepCapture(ctx, collector.StepContext{
			Name:                step.Name,
			Intent:              step.Intent,
			Command:             step.Command,
			ContainerCgroupPath: containerCgroupPath,
		})
		if err != nil {
			_ = stdoutCapture.Close()
			_ = stderrCapture.Close()
			return StepResult{}, nil, err
		}
	}

	start := time.Now()
	err = runCapturedCommand(ctx, "", m.binary, stdoutCapture, stderrCapture, "exec", containerName, "sh", "-lc", step.Command)
	duration := time.Since(start).Milliseconds()
	exitCode := exitCode(err)

	if closeErr := stdoutCapture.Close(); closeErr != nil {
		return StepResult{}, nil, fmt.Errorf("close stdout evidence %q: %w", stdoutPath, closeErr)
	}

	if closeErr := stderrCapture.Close(); closeErr != nil {
		return StepResult{}, nil, fmt.Errorf("close stderr evidence %q: %w", stderrPath, closeErr)
	}

	result := StepResult{
		Name:                 step.Name,
		Intent:               step.Intent,
		Command:              step.Command,
		ExitCode:             exitCode,
		DurationMilliseconds: duration,
		Stdout:               stdoutCapture.Output(),
		Stderr:               stderrCapture.Output(),
		Evidence: StepEvidence{
			Stdout: stdoutCapture.File(),
			Stderr: stderrCapture.File(),
		},
	}

	var stepEvents []collector.Event
	if stepCapture != nil {
		finishCtx, cancel := context.WithTimeout(context.Background(), defaultCollectorStopGrace)
		stepEvents, err = stepCapture.Finish(finishCtx)
		cancel()
		if err != nil {
			return result, stepEvents, err
		}
	}

	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return result, stepEvents, fmt.Errorf("docker exec %q: %w", step.Name, err)
	}

	return result, stepEvents, nil
}

func persistEvents(result *RunResult, evidenceDir string) error {
	if result == nil || len(result.Events) == 0 || strings.TrimSpace(result.EventsPath) != "" {
		return nil
	}

	eventsPath := filepath.Join(evidenceDir, "events.json")
	if err := writeJSON(eventsPath, result.Events); err != nil {
		return err
	}

	result.EventsPath = eventsPath
	result.EvidenceFiles = append(result.EvidenceFiles, eventsPath)
	return nil
}

func validateMountPath(kind string, value string) error {
	if strings.Contains(value, ":") {
		return fmt.Errorf("mount %s %q must not contain ':'", kind, value)
	}

	return nil
}

func (m Manager) containerCgroupPath(ctx context.Context, name string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, defaultInspectTimeout)
	defer cancel()

	stdout, stderr, err := runCommand(commandCtx, "", m.binary, "inspect", "-f", "{{.State.Pid}}", name)
	if err != nil {
		return "", fmt.Errorf("inspect container pid: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}

	pid, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return "", fmt.Errorf("parse container pid %q: %w", strings.TrimSpace(stdout), err)
	}

	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return "", fmt.Errorf("read container cgroup for pid %d: %w", pid, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(fields) != 3 || fields[0] != "0" || fields[1] != "" {
			continue
		}

		relativePath := strings.TrimSpace(fields[2])
		if relativePath == "" {
			break
		}

		cgroupPath := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(relativePath, "/"))
		if _, err := os.Stat(cgroupPath); err != nil {
			return "", fmt.Errorf("stat container cgroup %q: %w", cgroupPath, err)
		}

		return cgroupPath, nil
	}

	return "", errors.New("container is not running in a discoverable cgroup v2 hierarchy")
}

type outputCapture struct {
	path         string
	file         *os.File
	previewLimit int
	fileLimit    int64
	preview      bytes.Buffer
	totalBytes   int64
	storedBytes  int64
	truncated    bool
	fileTruncate bool
	hasher       hash.Hash
	writeErr     error
}

func newOutputCapture(path string, previewLimit int, fileLimit int64) (*outputCapture, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create evidence file %q: %w", path, err)
	}

	return &outputCapture{
		path:         path,
		file:         file,
		previewLimit: previewLimit,
		fileLimit:    fileLimit,
		hasher:       sha256.New(),
	}, nil
}

func (c *outputCapture) Write(data []byte) (int, error) {
	c.totalBytes += int64(len(data))
	if _, err := c.hasher.Write(data); err != nil && c.writeErr == nil {
		c.writeErr = err
	}

	if remaining := c.previewLimit - c.preview.Len(); remaining > 0 {
		chunk := data
		if len(chunk) > remaining {
			chunk = chunk[:remaining]
			c.truncated = true
		}
		_, _ = c.preview.Write(chunk)
	} else if len(data) > 0 {
		c.truncated = true
	}

	if c.file == nil || c.writeErr != nil {
		if c.file != nil && c.totalBytes > c.fileLimit {
			c.fileTruncate = true
		}
		return len(data), nil
	}

	remaining := c.fileLimit - c.storedBytes
	if remaining <= 0 {
		if len(data) > 0 {
			c.fileTruncate = true
		}
		return len(data), nil
	}

	chunk := data
	if int64(len(chunk)) > remaining {
		chunk = chunk[:remaining]
		c.fileTruncate = true
	}

	written, err := c.file.Write(chunk)
	c.storedBytes += int64(written)
	if err != nil && c.writeErr == nil {
		c.writeErr = err
		return len(data), nil
	}

	if written < len(chunk) && c.writeErr == nil {
		c.writeErr = io.ErrShortWrite
		return len(data), nil
	}

	if len(chunk) < len(data) {
		c.fileTruncate = true
	}

	return len(data), nil
}

func (c *outputCapture) Output() Output {
	return Output{
		Preview:    c.preview.String(),
		TotalBytes: c.totalBytes,
		SHA256:     hex.EncodeToString(c.hasher.Sum(nil)),
		Truncated:  c.truncated,
	}
}

func (c *outputCapture) File() OutputFile {
	return OutputFile{
		Path:        c.path,
		StoredBytes: c.storedBytes,
		Truncated:   c.fileTruncate,
	}
}

func (c *outputCapture) Close() error {
	if c.file == nil {
		return c.writeErr
	}

	err := c.file.Close()
	c.file = nil
	if c.writeErr != nil {
		return c.writeErr
	}

	return err
}

func runCapturedCommand(ctx context.Context, workdir string, binary string, stdout io.Writer, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	if strings.TrimSpace(workdir) != "" {
		cmd.Dir = workdir
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

func runCommand(ctx context.Context, workdir string, binary string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	if strings.TrimSpace(workdir) != "" {
		cmd.Dir = workdir
	}

	stdout := newLimitedStringCapture(lifecycleOutputLimitBytes)
	stderr := newLimitedStringCapture(lifecycleOutputLimitBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.String(), stderr.String(), err
}

func (m Manager) lifecycleTimeout(value time.Duration, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}

	return fallback
}

func (m Manager) runLifecycleCommand(ctx context.Context, timeout time.Duration, action string, args ...string) (string, string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout, stderr, err := runCommand(commandCtx, "", m.binary, args...)
	if err == nil {
		return stdout, stderr, nil
	}

	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		return stdout, stderr, fmt.Errorf("docker %s timed out after %s: %w", action, timeout, commandCtx.Err())
	}

	return stdout, stderr, fmt.Errorf("docker %s: %w", action, err)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

func sanitizeFilename(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}

	output := strings.Trim(builder.String(), "-")
	if output == "" {
		return "step"
	}

	return output
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode %q: %w", path, err)
	}

	return nil
}

type limitedStringCapture struct {
	limit     int
	buffer    bytes.Buffer
	truncated bool
}

func newLimitedStringCapture(limit int) limitedStringCapture {
	return limitedStringCapture{limit: limit}
}

func (c *limitedStringCapture) Write(data []byte) (int, error) {
	if c.limit <= 0 {
		return len(data), nil
	}

	remaining := c.limit - c.buffer.Len()
	switch {
	case remaining <= 0:
		if len(data) > 0 {
			c.truncated = true
		}
	case len(data) > remaining:
		_, _ = c.buffer.Write(data[:remaining])
		c.truncated = true
	default:
		_, _ = c.buffer.Write(data)
	}

	return len(data), nil
}

func (c limitedStringCapture) String() string {
	return c.buffer.String()
}
