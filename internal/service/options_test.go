package service

import (
	"reflect"
	"testing"
)

func TestProxyArgsBuildBackgroundProxyCommand(t *testing.T) {
	got := proxyArgs(Options{
		Addr:       "127.0.0.1:8788",
		Upstream:   "https://openrouter.ai/api",
		PolicyPath: "/tmp/policy.json",
	})
	want := []string{
		"proxy",
		"--addr", "127.0.0.1:8788",
		"--upstream", "https://openrouter.ai/api",
		"--policy", "/tmp/policy.json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("proxyArgs() = %#v, want %#v", got, want)
	}
}

func TestParseActionRejectsUnknownAction(t *testing.T) {
	if _, err := ParseAction("reload"); err == nil {
		t.Fatal("ParseAction accepted unknown action")
	}
}
