package service

import (
	"fmt"
	"os"
	"strings"
)

func validateOptions(opts Options) error {
	if strings.TrimSpace(opts.BinaryPath) == "" {
		return fmt.Errorf("binary path is required")
	}
	if strings.TrimSpace(opts.Addr) == "" {
		return fmt.Errorf("listen address is required")
	}
	if strings.TrimSpace(opts.Upstream) == "" {
		return fmt.Errorf("upstream URL is required")
	}
	return nil
}

func requireInstalled(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("service not installed (missing %s); run: veil service install", path)
		}
		return err
	}
	return nil
}

func proxyArgs(opts Options) []string {
	args := []string{"proxy", "--addr", opts.Addr, "--upstream", opts.Upstream}
	if strings.TrimSpace(opts.PolicyPath) != "" {
		args = append(args, "--policy", strings.TrimSpace(opts.PolicyPath))
	}
	return args
}
