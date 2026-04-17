package collector

import (
	"context"
	"fmt"
	"time"
)

type Event struct {
	Kind      string    `json:"kind"`
	Step      string    `json:"step,omitempty"`
	Command   string    `json:"command,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type RunContext struct {
	Side     string
	RunIndex int
}

type StepContext struct {
	Name                string
	Intent              string
	Command             string
	ContainerCgroupPath string
}

type Factory interface {
	Start(RunContext) (RunCollector, error)
}

type RunCollector interface {
	StartStepCapture(context.Context, StepContext) (StepCapture, error)
}

type StepCapture interface {
	Finish(context.Context) ([]Event, error)
}

type CaptureError struct {
	Op   string
	Step string
	Err  error
}

func (e *CaptureError) Error() string {
	if e == nil {
		return ""
	}
	if e.Step != "" {
		return fmt.Sprintf("%s for step %q: %v", e.Op, e.Step, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *CaptureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewFactory() Factory {
	return newFactory()
}
