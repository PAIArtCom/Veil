// Command opencloak is the entry point for running the OpenCloak engine through
// one of its transports.
//
// Usage:
//
//	opencloak <command> [flags]
//
// Commands:
//
//	proxy     run the base-URL local proxy (Claude Code; Codex planned)
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
	"os"
	"os/signal"
	"syscall"
	"time"

	opencloak "github.com/cloakia/opencloak"
	"github.com/cloakia/opencloak/internal/proxy"
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
			fmt.Fprintf(os.Stderr, "opencloak proxy: %v\n", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		printVersion(os.Stdout)
	case "serve", "console", "mask":
		fmt.Fprintf(os.Stderr, "opencloak %s: not implemented yet\n", os.Args[1])
		os.Exit(1)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "opencloak: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func printVersion(w io.Writer) {
	fmt.Fprintln(w, versionString())
}

func versionString() string {
	return fmt.Sprintf("opencloak %s (commit %s, built %s)", version, commit, buildDate)
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

	engine, err := opencloak.New(opencloak.Config{})
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

	fmt.Fprintf(stderr, "opencloak proxy listening on http://%s\n", *addr)
	fmt.Fprintf(stderr, "  upstream: %s\n", *upstream)
	fmt.Fprintf(stderr, "  point your tool at it:  set ANTHROPIC_BASE_URL=http://%s\n", *addr)

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
		fmt.Fprintln(stderr, "opencloak proxy: shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
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
	fmt.Fprint(os.Stderr, `opencloak — de-identification engine for AI coding tools

usage: opencloak <command> [flags]

commands:
  proxy     run the base-URL local proxy (Claude Code; Codex planned)
  version   print build version metadata
  serve     run the HTTP/gRPC service (Phase 1)
  console   run the local web console (localhost-only)
  mask      one-shot mask/restore utility for testing

proxy flags:
  --addr      loopback listen address (default 127.0.0.1:8787)
  --upstream  upstream provider base URL (default https://api.anthropic.com)
`)
}
