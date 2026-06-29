package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	veil "github.com/PAIArtCom/Veil"
	"github.com/PAIArtCom/Veil/internal/service"
)

func TestVersionStringDefaultsAreStable(t *testing.T) {
	got := versionString()
	want := "veil v0.1.0-dev (commit unknown, built unknown)"
	if got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

// TestIsLoopbackAddr covers the loopback-only bind guard that satisfies the
// "binds 127.0.0.1 only" invariant.
func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8787", true},
		{"127.0.0.1", true},
		{"127.5.6.7:80", true}, // anywhere in 127.0.0.0/8
		{"localhost:1", true},
		{"localhost", true},
		{"[::1]:80", true},
		{"::1", true},
		{"0.0.0.0:8787", false},
		{"192.168.1.5:80", false},
		{"10.0.0.1:443", false},
		{"example.com:443", false},
		{":8787", false}, // empty host binds all interfaces
		{"[::]:8787", false},
	}
	for _, c := range cases {
		if got := isLoopbackAddr(c.addr); got != c.want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", c.addr, got, c.want)
		}
	}
}

// TestRunProxyRejectsNonLoopback proves runProxy returns an error (without
// binding a socket) when --addr is not loopback. No listener is opened because
// the guard fires before ListenAndServe.
func TestRunProxyRejectsNonLoopback(t *testing.T) {
	var buf bytes.Buffer
	err := runProxy([]string{"--addr", "0.0.0.0:8787"}, &buf)
	if err == nil {
		t.Fatal("runProxy with non-loopback addr returned nil error")
	}
	if !strings.Contains(err.Error(), "non-loopback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunProxyRejectsBadUpstream proves runProxy validates the upstream URL and
// fails before binding when it is malformed.
func TestRunProxyRejectsBadUpstream(t *testing.T) {
	var buf bytes.Buffer
	err := runProxy([]string{"--addr", "127.0.0.1:0", "--upstream", "ftp://nope"}, &buf)
	if err == nil {
		t.Fatal("runProxy with bad upstream returned nil error")
	}
	if !strings.Contains(err.Error(), "init proxy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnginePolicyProviderNilUsesBuiltInDefaults(t *testing.T) {
	var provider veil.PolicyProvider = enginePolicyProvider(nil)
	if provider != nil {
		t.Fatal("nil local provider became a non-nil PolicyProvider interface")
	}
}

func TestRunProxyRejectsMissingPolicyPathBeforeBinding(t *testing.T) {
	var buf bytes.Buffer
	err := runProxy([]string{"--addr", "127.0.0.1:0", "--policy", filepath.Join(t.TempDir(), "missing.json")}, &buf)
	if err == nil {
		t.Fatal("runProxy with missing policy returned nil error")
	}
	if !strings.Contains(err.Error(), "load policy") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunProxyRejectsInvalidPolicyBeforeBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"types":{"SECRET":{"operator":"redact"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := runProxy([]string{"--addr", "127.0.0.1:0", "--policy", path}, &buf)
	if err == nil {
		t.Fatal("runProxy with invalid policy returned nil error")
	}
	if !strings.Contains(err.Error(), "load policy") || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunProxyBadFlag ensures unknown flags surface a parse error rather than
// panicking or binding.
func TestRunProxyBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := runProxy([]string{"--nope"}, &buf); err == nil {
		t.Fatal("runProxy with unknown flag returned nil error")
	}
}

func TestRunProxyHelpReturnsSuccess(t *testing.T) {
	var buf bytes.Buffer
	if err := runProxy([]string{"--help"}, &buf); err != nil {
		t.Fatalf("runProxy --help returned error: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "Usage of proxy") || !strings.Contains(got, "-addr") || !strings.Contains(got, "-policy") {
		t.Fatalf("help output missing expected flags: %q", got)
	}
}

func TestSplitServiceArgsAllowsActionBeforeOrAfterFlags(t *testing.T) {
	action, flags, err := splitServiceArgs([]string{"install", "--addr", "127.0.0.1:8788", "--force"})
	if err != nil {
		t.Fatalf("split action-first: %v", err)
	}
	if action != "install" || strings.Join(flags, " ") != "--addr 127.0.0.1:8788 --force" {
		t.Fatalf("action=%q flags=%q", action, flags)
	}

	action, flags, err = splitServiceArgs([]string{"--force", "--upstream=https://api.openai.com", "restart"})
	if err != nil {
		t.Fatalf("split flags-first: %v", err)
	}
	if action != "restart" || strings.Join(flags, " ") != "--force --upstream=https://api.openai.com" {
		t.Fatalf("action=%q flags=%q", action, flags)
	}
}

func TestRunServiceDryRunInstallBuildsBackgroundProxyPlan(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runService([]string{
		"install",
		"--dry-run",
		"--force",
		"--bin", filepath.Join(t.TempDir(), "veil"),
		"--addr", "127.0.0.1:8788",
		"--upstream", "https://api.openai.com",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runService dry-run: %v\nstderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "write") || !strings.Contains(out, "com.paiart.veil") && !strings.Contains(out, "veil.service") {
		t.Fatalf("dry-run output missing service definition write: %s", out)
	}
	if !strings.Contains(out, "launchctl") && !strings.Contains(out, "systemctl") {
		t.Fatalf("dry-run output missing service-manager command: %s", out)
	}
	if !strings.Contains(out, "Dry run complete") || !strings.Contains(out, "without --dry-run") {
		t.Fatalf("dry-run output missing user guidance: %s", out)
	}
}

func TestRunServiceRejectsNonLoopbackAddr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runService([]string{
		"install",
		"--dry-run",
		"--bin", filepath.Join(t.TempDir(), "veil"),
		"--addr", "0.0.0.0:8788",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runService accepted non-loopback address")
	}
	if !strings.Contains(err.Error(), "non-loopback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunStatusReportsRunningProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("path = %q, want /healthz", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	var stdout, stderr bytes.Buffer
	if err := runStatus([]string{"--addr", addr}, &stdout, &stderr); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	if !strings.Contains(stdout.String(), "running") || !strings.Contains(stdout.String(), `"status":"ok"`) {
		t.Fatalf("status output missing running health: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Claude Code") || !strings.Contains(stdout.String(), "http://"+addr+"/v1") {
		t.Fatalf("status output missing setup guidance: %s", stdout.String())
	}
}

func TestPrintServiceNextStepsInstallGuidesUser(t *testing.T) {
	var stdout bytes.Buffer
	printServiceNextSteps(&stdout, "install", serviceOptionsForTest("127.0.0.1:8787"))
	out := stdout.String()
	for _, want := range []string{
		"Veil service installed and started.",
		"veil status",
		"~/.claude/settings.json",
		"http://127.0.0.1:8787/v1",
		"/veil/upstream=https://openrouter.ai/api/v1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("install guidance missing %q:\n%s", want, out)
		}
	}
}

func serviceOptionsForTest(addr string) service.Options {
	return service.Options{Addr: addr}
}
