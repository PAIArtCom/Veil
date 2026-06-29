//go:build !darwin && !linux && !windows

package service

import "fmt"

type unsupportedManager struct{}

func DefaultManager() Manager {
	return unsupportedManager{}
}

func (unsupportedManager) Plan(action Action, opts Options) (*Plan, error) {
	return nil, fmt.Errorf("veil service is not supported on this OS yet; run veil proxy manually")
}
