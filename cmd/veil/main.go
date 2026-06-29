// Command veil is the entry point for running the Veil engine through
// one of its transports.
//
// Usage:
//
//	veil <command> [flags]
//
// Commands:
//
//	proxy     run the base-URL local proxy (Claude Code and Codex Responses)
//	service   install/start/stop/restart/status the background proxy service
//	status    check whether the local proxy is reachable
//	restart   restart the background proxy service
//	version   print build version metadata
//	serve     run the HTTP/gRPC service (Phase 1)
//	console   run the local web console (localhost-only)
//	mask      one-shot mask/restore utility for testing
//
// Only proxy is implemented in Phase 0; the others print "not implemented yet".
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	veil "github.com/PAIArtCom/Veil"
	localconfig "github.com/PAIArtCom/Veil/internal/config"
	"github.com/PAIArtCom/Veil/internal/proxy"
	"github.com/PAIArtCom/Veil/internal/service"
)

var (
	version   = "v0.1.0-dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "proxy":
		if err := runProxy(os.Args[2:], os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "veil proxy: %v\n", err)
			os.Exit(1)
		}
	case "service":
		if err := runService(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "veil service: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "veil status: %v\n", err)
			os.Exit(1)
		}
	case "restart":
		if err := runService([]string{"restart"}, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "veil restart: %v\n", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		printVersion(os.Stdout)
	case "serve", "console", "mask":
		fmt.Fprintf(os.Stderr, "veil %s: not implemented yet\n", os.Args[1])
		os.Exit(1)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "veil: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func printVersion(w io.Writer) {
	fmt.Fprintln(w, versionString())
}

func versionString() string {
	return fmt.Sprintf("veil %s (commit %s, built %s)", version, commit, buildDate)
}

// runProxy parses the proxy subcommand flags, enforces the loopback-only bind
// invariant, constructs the engine and proxy handler, and serves until SIGINT/
// SIGTERM. It is factored out of main so the flag parsing and the loopback guard
// are unit-testable without binding a socket (an invalid --addr returns an error
// before ListenAndServe is reached). stderr receives the startup banner.
func runProxy(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8787", "loopback address to listen on (host must be a loopback address)")
	upstream := fs.String("upstream", "https://api.anthropic.com", "upstream provider base URL")
	policyPath := fs.String("policy", "", "local policy JSON path (default: VEIL_POLICY or ~/.veil/policy.json if present)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Loopback-only binding is a hard invariant from the threat model and
	// ADR-0004: the proxy forwards the client's credential verbatim, so exposing
	// it off-host would let any reachable client borrow that credential. Reject
	// non-loopback addresses before opening a listener.
	if !isLoopbackAddr(*addr) {
		return fmt.Errorf("refusing to bind non-loopback address %q: the proxy passes the client credential through and must bind a loopback address (127.0.0.0/8, ::1, or localhost)", *addr)
	}

	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	policyProvider, policySource, err := localconfig.LoadProvider(localconfig.LoadOptions{Path: *policyPath})
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}

	engine, err := veil.New(veil.Config{Policy: enginePolicyProvider(policyProvider)})
	if err != nil {
		return fmt.Errorf("init engine: %w", err)
	}

	px, err := proxy.New(engine, *upstream, logger)
	if err != nil {
		return fmt.Errorf("init proxy: %w", err)
	}

	server := &http.Server{
		Addr:    *addr,
		Handler: px,
		// No ReadTimeout/WriteTimeout: streaming SSE responses are long-lived.
		// ReadHeaderTimeout guards against a slow-loris client holding a
		// connection open without sending headers.
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stderr, "veil proxy listening on http://%s\n", *addr)
	fmt.Fprintf(stderr, "  upstream: %s\n", *upstream)
	if policySource.Loaded {
		fmt.Fprintf(stderr, "  policy: loaded from %s\n", policySource.From)
	} else {
		fmt.Fprintln(stderr, "  policy: built-in defaults")
	}
	fmt.Fprintf(stderr, "  Claude Code: set ~/.claude/settings.json env.ANTHROPIC_BASE_URL=\"http://%s\"\n", *addr)
	fmt.Fprintf(stderr, "  Codex CLI: set model_providers.<name>.base_url=\"http://%s/v1\"\n", *addr)
	fmt.Fprintf(stderr, "  Dynamic upstream: use http://%s/veil/upstream=https://provider.example/v1\n", *addr)

	// Graceful shutdown on SIGINT/SIGTERM: stop accepting, drain in-flight.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		fmt.Fprintln(stderr, "veil proxy: shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}

func runService(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || isHelpToken(args[0]) {
		printServiceUsage(stdout)
		return nil
	}

	actionArg, flagArgs, err := splitServiceArgs(args)
	if err != nil {
		printServiceUsage(stderr)
		return err
	}

	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8787", "loopback address the background proxy listens on")
	upstream := fs.String("upstream", "https://api.anthropic.com", "default upstream provider base URL")
	policyPath := fs.String("policy", "", "local policy JSON path for the background proxy")
	binaryPath := fs.String("bin", "", "path to veil binary (default: current executable)")
	force := fs.Bool("force", false, "overwrite an existing service definition")
	dryRun := fs.Bool("dry-run", false, "print service-manager actions without executing them")
	timeout := fs.Duration("timeout", 30*time.Second, "overall timeout for service-manager commands")
	stdoutPath := fs.String("stdout", defaultLogPath("out"), "macOS launchd stdout log path")
	stderrPath := fs.String("stderr", defaultLogPath("err"), "macOS launchd stderr log path")
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printServiceUsage(stdout)
			return nil
		}
		return err
	}

	action, err := service.ParseAction(actionArg)
	if err != nil {
		printServiceUsage(stderr)
		return err
	}
	if !isLoopbackAddr(*addr) {
		return fmt.Errorf("refusing non-loopback service address %q", *addr)
	}
	if u, err := url.Parse(*upstream); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid upstream URL %q", *upstream)
	}

	bin := *binaryPath
	if strings.TrimSpace(bin) == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determine executable path: %w", err)
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		bin = exe
	}

	opts := service.Options{
		Addr:       *addr,
		Upstream:   *upstream,
		PolicyPath: *policyPath,
		BinaryPath: bin,
		Force:      *force,
		DryRun:     *dryRun,
		StdoutPath: *stdoutPath,
		StderrPath: *stderrPath,
	}
	plan, err := service.DefaultManager().Plan(action, opts)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	out, err := service.ExecutePlan(ctx, plan, opts.DryRun)
	if out != "" {
		fmt.Fprint(stdout, out)
	}
	if err != nil {
		return err
	}
	printServiceNextSteps(stdout, action, opts)
	return nil
}

func runStatus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8787", "loopback address to check")
	timeout := fs.Duration("timeout", 2*time.Second, "HTTP probe timeout")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if !isLoopbackAddr(*addr) {
		return fmt.Errorf("refusing non-loopback status address %q", *addr)
	}

	target := "http://" + *addr + "/healthz"
	client := &http.Client{Timeout: *timeout}
	resp, err := client.Get(target)
	if err != nil {
		fmt.Fprintf(stdout, "Veil proxy: not reachable at %s\n", target)
		fmt.Fprintln(stdout, "Hint: run `veil service install` once, then `veil status` again.")
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Veil proxy: reachable at %s, but returned HTTP %d\n", target, resp.StatusCode)
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(stdout, "Veil proxy: running at http://%s\n", *addr)
	if len(body) > 0 {
		fmt.Fprintf(stdout, "%s", body)
	}
	fmt.Fprintln(stdout)
	printAgentConfigHint(stdout, *addr)
	return nil
}

func printServiceNextSteps(w io.Writer, action service.Action, opts service.Options) {
	addr := strings.TrimSpace(opts.Addr)
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	if opts.DryRun {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Dry run complete. No service changes were made.")
		fmt.Fprintln(w, "Next: re-run the same command without --dry-run to apply it.")
		return
	}

	switch action {
	case service.ActionInstall:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Veil service installed and started.")
		fmt.Fprintln(w, "Next: run `veil status`, then configure your AI tool with this local base URL.")
		printAgentConfigHint(w, addr)
	case service.ActionStart:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Veil service started.")
		fmt.Fprintln(w, "Next: run `veil status` to confirm the local proxy is reachable.")
	case service.ActionRestart:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Veil service restarted.")
		fmt.Fprintln(w, "Next: run `veil status` to confirm the new settings are active.")
	case service.ActionStop:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Veil service stopped.")
		fmt.Fprintln(w, "Start it again with: veil service start")
	case service.ActionUninstall:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Veil service removed.")
		fmt.Fprintln(w, "Next: remove the Veil base URL from Claude Code or Codex if you no longer want to use it.")
	case service.ActionStatus:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "For proxy health, run: veil status")
	}
}

func printAgentConfigHint(w io.Writer, addr string) {
	fmt.Fprintln(w, "Claude Code: set ~/.claude/settings.json env.ANTHROPIC_BASE_URL to:")
	fmt.Fprintf(w, "  http://%s\n", addr)
	fmt.Fprintln(w, "Codex CLI: set ~/.codex/config.toml model provider base_url to:")
	fmt.Fprintf(w, "  http://%s/v1\n", addr)
	fmt.Fprintln(w, "OpenRouter via Codex:")
	fmt.Fprintf(w, "  http://%s/veil/upstream=https://openrouter.ai/api/v1\n", addr)
}

func enginePolicyProvider(provider *localconfig.Provider) veil.PolicyProvider {
	if provider == nil {
		return nil
	}
	return provider
}

// isLoopbackAddr reports whether addr (a "host:port" listen address) binds a
// loopback interface only. It accepts the literal "localhost", any address in
// 127.0.0.0/8, and the IPv6 loopback ::1. A bare host without a port is also
// accepted so callers can validate either form.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port present (e.g. "127.0.0.1" or "localhost"): treat addr as host.
		host = addr
	}
	if host == "" {
		// e.g. ":8787" binds all interfaces — not loopback.
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	// A non-IP, non-"localhost" hostname (e.g. example.com) is not loopback.
	return false
}

func usage() {
	fmt.Fprint(os.Stderr, `veil — de-identification engine for AI coding tools

usage: veil <command> [flags]

commands:
  proxy     run the base-URL local proxy (Claude Code and Codex Responses)
  service   install/start/stop/restart/status the background proxy service
  status    check whether the local proxy is reachable
  restart   restart the background proxy service
  version   print build version metadata
  serve     run the HTTP/gRPC service (Phase 1)
  console   run the local web console (localhost-only)
  mask      one-shot mask/restore utility for testing

proxy flags:
  --addr      loopback listen address (default 127.0.0.1:8787)
  --upstream  upstream provider base URL (default https://api.anthropic.com)
  --policy    local policy JSON path (default VEIL_POLICY or ~/.veil/policy.json)

service examples:
  veil service install
  veil service install --addr 127.0.0.1:8788 --upstream https://api.openai.com
  veil service restart
  veil status
`)
}

func printServiceUsage(w io.Writer) {
	fmt.Fprint(w, `usage: veil service [flags] <install|uninstall|start|stop|restart|status>

examples:
  veil service install
  veil service install --addr 127.0.0.1:8788 --upstream https://api.openai.com
  veil service install --force
  veil service status
  veil restart

flags:
  --addr      loopback listen address for the background proxy (default 127.0.0.1:8787)
  --upstream  default upstream provider base URL (default https://api.anthropic.com)
  --policy    local policy JSON path
  --force     overwrite an existing service definition
  --dry-run   print service-manager actions without executing them
`)
}

func isHelpToken(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func splitServiceArgs(args []string) (action string, flagArgs []string, err error) {
	needsValue := map[string]bool{
		"addr":     true,
		"upstream": true,
		"policy":   true,
		"bin":      true,
		"timeout":  true,
		"stdout":   true,
		"stderr":   true,
	}

	var haveAction bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing action")
			}
			if haveAction {
				return "", nil, fmt.Errorf("unexpected argument %q", args[i+1])
			}
			action = args[i+1]
			haveAction = true
			if i+2 < len(args) {
				return "", nil, fmt.Errorf("unexpected argument %q", args[i+2])
			}
			break
		}
		if !strings.HasPrefix(a, "-") {
			if haveAction {
				return "", nil, fmt.Errorf("unexpected argument %q", a)
			}
			action = a
			haveAction = true
			continue
		}
		flagArgs = append(flagArgs, a)
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if needsValue[name] && !strings.Contains(a, "=") {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("flag %s needs a value", a)
			}
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	if !haveAction {
		return "", nil, fmt.Errorf("missing action")
	}
	return action, flagArgs, nil
}

func defaultLogPath(kind string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".veil", "logs", "veil."+kind+".log")
}
