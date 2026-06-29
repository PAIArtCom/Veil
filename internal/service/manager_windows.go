//go:build windows

package service

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

const windowsTaskName = "Veil"

type windowsManager struct{}

func DefaultManager() Manager {
	return windowsManager{}
}

func (windowsManager) Plan(action Action, opts Options) (*Plan, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	runLine := buildWindowsTaskRunLine(opts)
	plan := &Plan{}
	switch action {
	case ActionInstall:
		runAs := currentWindowsUser()
		createArgs := []string{"/Create", "/TN", windowsTaskName, "/TR", runLine, "/SC", "ONLOGON"}
		if runAs != "" {
			createArgs = append(createArgs, "/RU", runAs, "/IT")
		}
		createArgs = append(createArgs, "/RL", "LIMITED")
		if opts.Force {
			createArgs = append(createArgs, "/F")
		}
		plan.Commands = append(plan.Commands,
			Command{Path: "schtasks.exe", Args: createArgs},
			Command{Path: "schtasks.exe", Args: []string{"/Run", "/TN", windowsTaskName}},
		)
	case ActionUninstall:
		plan.Commands = append(plan.Commands,
			Command{Path: "schtasks.exe", Args: []string{"/End", "/TN", windowsTaskName}, IgnoreError: true},
			Command{Path: "schtasks.exe", Args: []string{"/Delete", "/TN", windowsTaskName, "/F"}},
		)
	case ActionStart:
		plan.Commands = append(plan.Commands, Command{Path: "schtasks.exe", Args: []string{"/Run", "/TN", windowsTaskName}})
	case ActionStop:
		plan.Commands = append(plan.Commands, Command{Path: "schtasks.exe", Args: []string{"/End", "/TN", windowsTaskName}, IgnoreError: true})
	case ActionRestart:
		plan.Commands = append(plan.Commands,
			Command{Path: "schtasks.exe", Args: []string{"/End", "/TN", windowsTaskName}, IgnoreError: true},
			Command{Path: "schtasks.exe", Args: []string{"/Run", "/TN", windowsTaskName}},
		)
	case ActionStatus:
		plan.Commands = append(plan.Commands, Command{Path: "schtasks.exe", Args: []string{"/Query", "/TN", windowsTaskName, "/FO", "LIST", "/V"}})
	default:
		return nil, fmt.Errorf("unsupported action %q", action)
	}

	return plan, nil
}

func currentWindowsUser() string {
	if u, err := user.Current(); err == nil {
		if name := strings.TrimSpace(u.Username); name != "" {
			return name
		}
	}
	userName := strings.TrimSpace(os.Getenv("USERNAME"))
	if userName == "" {
		return ""
	}
	domain := strings.TrimSpace(os.Getenv("USERDOMAIN"))
	if domain != "" {
		return domain + `\` + userName
	}
	return userName
}

func buildWindowsTaskRunLine(opts Options) string {
	parts := append([]string{quoteWindowsCmd(opts.BinaryPath)}, proxyArgs(opts)...)
	for i := 1; i < len(parts); i++ {
		parts[i] = quoteWindowsCmd(parts[i])
	}
	return strings.Join(parts, " ")
}

func quoteWindowsCmd(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return `""`
	}
	escaped := strings.ReplaceAll(s, `"`, `\"`)
	return `"` + escaped + `"`
}
