//go:build !linux

package collector

import "errors"

type unsupportedFactory struct{}

func newFactory() Factory {
	return unsupportedFactory{}
}

func (unsupportedFactory) Start(RunContext) (RunCollector, error) {
	return nil, &CaptureError{
		Op:  "start host signal collector",
		Err: errors.New("real host signal capture requires Linux"),
	}
}
