package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionStringDefaultsAreStable(t *testing.T) {
	got := versionString()
	want := "opencloak v0.1.0-dev (commit unknown, built unknown)"
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
	if got := buf.String(); !strings.Contains(got, "Usage of proxy") || !strings.Contains(got, "-addr") {
		t.Fatalf("help output missing expected flags: %q", got)
	}
}
