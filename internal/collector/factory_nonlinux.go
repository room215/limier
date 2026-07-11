//go:build !linux

package collector

import "errors"

type unsupportedFactory struct{}

func newFactory() Factory {
	return unsupportedFactory{}
}

func (unsupportedFactory) Start(RunContext) (RunCollector, error) {
	return nil, &CaptureError{
		Op:  "start telemetry collector",
		Err: errors.New("kernel telemetry capture requires Linux"),
	}
}
