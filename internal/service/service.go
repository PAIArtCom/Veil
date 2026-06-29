package service

import "fmt"

type Action string

const (
	ActionInstall   Action = "install"
	ActionUninstall Action = "uninstall"
	ActionStart     Action = "start"
	ActionStop      Action = "stop"
	ActionRestart   Action = "restart"
	ActionStatus    Action = "status"
)

func ParseAction(s string) (Action, error) {
	switch Action(s) {
	case ActionInstall, ActionUninstall, ActionStart, ActionStop, ActionRestart, ActionStatus:
		return Action(s), nil
	default:
		return "", fmt.Errorf("unknown action %q", s)
	}
}

type Options struct {
	Addr       string
	Upstream   string
	PolicyPath string
	BinaryPath string
	Force      bool
	DryRun     bool
	StdoutPath string
	StderrPath string
}

type Manager interface {
	Plan(action Action, opts Options) (*Plan, error)
}
