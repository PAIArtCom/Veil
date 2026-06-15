// Package proxy is the standalone base-URL local proxy transport. It binds 127.0.0.1
// only, passes the client's credential header through unchanged, and rewrites only
// the JSON body. See docs/architecture/decisions/0001-base-url-proxy-over-hooks.md
// and 0004-auth-pass-through.md.
//
// Status: scaffold only; no behavior yet.
package proxy
