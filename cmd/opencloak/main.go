// Command opencloak is the planned entry point for running the OpenCloak engine through
// one of its transports.
//
// Usage:
//
//	opencloak <command> [flags]
//
// Commands:
//
//	proxy     run the base-URL local proxy (Claude Code; Codex planned)
//	serve     run the HTTP/gRPC service (Phase 1)
//	console   run the local web console (localhost-only)
//	mask      one-shot mask/restore utility for testing
//
// Status: scaffold only - commands are not implemented yet.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "proxy", "serve", "console", "mask":
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

func usage() {
	fmt.Fprint(os.Stderr, `opencloak — de-identification engine for AI coding tools

usage: opencloak <command> [flags]

commands:
  proxy     run the base-URL local proxy (Claude Code; Codex planned)
  serve     run the HTTP/gRPC service (Phase 1)
  console   run the local web console (localhost-only)
  mask      one-shot mask/restore utility for testing
`)
}
