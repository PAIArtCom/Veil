// Package service builds and executes OS user-service plans for keeping the
// Veil localhost proxy running in the background.
//
// It owns service-manager integration only. The network proxy behavior remains
// in internal/proxy, and command-line parsing remains in cmd/veil.
package service
