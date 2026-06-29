//go:build darwin

package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const darwinLaunchdLabel = "com.paiart.veil"

type darwinManager struct{}

func DefaultManager() Manager {
	return darwinManager{}
}

func (darwinManager) Plan(action Action, opts Options) (*Plan, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	plistPath := filepath.Join(launchAgentsDir, darwinLaunchdLabel+".plist")

	target := fmt.Sprintf("gui/%d", os.Getuid())
	serviceID := fmt.Sprintf("%s/%s", target, darwinLaunchdLabel)

	if action == ActionInstall {
		if _, err := os.Stat(plistPath); err == nil && !opts.Force {
			return nil, fmt.Errorf("service already installed (%s); re-run with --force to overwrite", plistPath)
		}
	} else if err := requireInstalled(plistPath); err != nil {
		return nil, err
	}

	plan := &Plan{}
	switch action {
	case ActionInstall:
		content := buildLaunchdPlist(opts)
		plan.Mkdirs = append(plan.Mkdirs, launchAgentsDir)
		plan.Writes = append(plan.Writes, FileWrite{Path: plistPath, Content: []byte(content), Perm: 0o644})
		if opts.Force {
			plan.Commands = append(plan.Commands, Command{Path: "launchctl", Args: []string{"bootout", target, plistPath}, IgnoreError: true})
		}
		plan.Commands = append(plan.Commands,
			Command{Path: "launchctl", Args: []string{"bootstrap", target, plistPath}},
			Command{Path: "launchctl", Args: []string{"kickstart", "-k", serviceID}},
		)
	case ActionUninstall:
		plan.Commands = append(plan.Commands, Command{Path: "launchctl", Args: []string{"bootout", target, plistPath}, IgnoreError: true})
		plan.Removes = append(plan.Removes, plistPath)
	case ActionStart:
		plan.Commands = append(plan.Commands,
			Command{Path: "launchctl", Args: []string{"bootstrap", target, plistPath}, IgnoreError: true},
			Command{Path: "launchctl", Args: []string{"kickstart", "-k", serviceID}},
		)
	case ActionStop:
		plan.Commands = append(plan.Commands, Command{Path: "launchctl", Args: []string{"bootout", target, plistPath}, IgnoreError: true})
	case ActionRestart:
		plan.Commands = append(plan.Commands,
			Command{Path: "launchctl", Args: []string{"bootout", target, plistPath}, IgnoreError: true},
			Command{Path: "launchctl", Args: []string{"bootstrap", target, plistPath}},
			Command{Path: "launchctl", Args: []string{"kickstart", "-k", serviceID}},
		)
	case ActionStatus:
		plan.Commands = append(plan.Commands, Command{Path: "launchctl", Args: []string{"print", serviceID}})
	default:
		return nil, fmt.Errorf("unsupported action %q", action)
	}

	return plan, nil
}

func buildLaunchdPlist(opts Options) string {
	args := proxyArgs(opts)
	var b strings.Builder
	_, _ = b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	_, _ = b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	_, _ = b.WriteString(`<plist version="1.0">` + "\n")
	_, _ = b.WriteString(`  <dict>` + "\n")
	_, _ = b.WriteString(`    <key>Label</key>` + "\n")
	_, _ = b.WriteString(`    <string>` + xmlEscape(darwinLaunchdLabel) + `</string>` + "\n\n")
	_, _ = b.WriteString(`    <key>ProgramArguments</key>` + "\n")
	_, _ = b.WriteString(`    <array>` + "\n")
	_, _ = b.WriteString(`      <string>` + xmlEscape(opts.BinaryPath) + `</string>` + "\n")
	for _, arg := range args {
		_, _ = b.WriteString(`      <string>` + xmlEscape(arg) + `</string>` + "\n")
	}
	_, _ = b.WriteString(`    </array>` + "\n\n")
	_, _ = b.WriteString(`    <key>RunAtLoad</key>` + "\n")
	_, _ = b.WriteString(`    <true/>` + "\n")
	_, _ = b.WriteString(`    <key>KeepAlive</key>` + "\n")
	_, _ = b.WriteString(`    <true/>` + "\n")

	if strings.TrimSpace(opts.StdoutPath) != "" {
		_, _ = b.WriteString("\n")
		_, _ = b.WriteString(`    <key>StandardOutPath</key>` + "\n")
		_, _ = b.WriteString(`    <string>` + xmlEscape(strings.TrimSpace(opts.StdoutPath)) + `</string>` + "\n")
	}
	if strings.TrimSpace(opts.StderrPath) != "" {
		_, _ = b.WriteString("\n")
		_, _ = b.WriteString(`    <key>StandardErrorPath</key>` + "\n")
		_, _ = b.WriteString(`    <string>` + xmlEscape(strings.TrimSpace(opts.StderrPath)) + `</string>` + "\n")
	}

	_, _ = b.WriteString(`  </dict>` + "\n")
	_, _ = b.WriteString(`</plist>` + "\n")
	return b.String()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
