package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	opencloak "github.com/cloakia/opencloak"
	"github.com/cloakia/opencloak/internal/types"
)

func TestLoadProviderValidConfig(t *testing.T) {
	path := writePolicy(t, `{
		"default_operator": "token",
		"types": {
			"EMAIL": {"operator": "ignore"},
			"SECRET": {"operator": "block"}
		}
	}`)

	provider, source, err := LoadProvider(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	if provider == nil || !source.Loaded || source.From != "flag" {
		t.Fatalf("unexpected provider/source: provider=%v source=%+v", provider, source)
	}

	policy, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	if policy.DefaultOperator != types.OperatorToken {
		t.Fatalf("DefaultOperator = %q", policy.DefaultOperator)
	}
	if got := policy.Types[types.TypeEmail].Operator; got != types.OperatorIgnore {
		t.Fatalf("EMAIL operator = %q", got)
	}
	if got := policy.Types[types.TypeSecret].Operator; got != types.OperatorBlock {
		t.Fatalf("SECRET operator = %q", got)
	}
}

func TestLoadProviderDefaultToken(t *testing.T) {
	path := writePolicy(t, `{}`)
	provider, _, err := LoadProvider(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	policy, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	if policy.DefaultOperator != types.OperatorToken {
		t.Fatalf("default operator = %q, want token", policy.DefaultOperator)
	}
	if len(policy.Types) != 0 {
		t.Fatalf("types = %+v, want none", policy.Types)
	}
}

func TestLoadProviderRejectsEmptyDefaultOperator(t *testing.T) {
	assertInvalidPolicy(t, `{"default_operator":""}`, "default_operator must be one of")
}

func TestLoadProviderRejectsAllIgnorePolicy(t *testing.T) {
	assertInvalidPolicy(t, `{"default_operator":"ignore"}`, "ignores every supported sensitive type")
}

func TestLoadProviderRejectsAllEffectiveTypesIgnored(t *testing.T) {
	assertInvalidPolicy(t, `{
		"default_operator": "ignore",
		"types": {
			"SECRET": {"operator": "ignore"},
			"EMAIL": {"operator": "ignore"}
		}
	}`, "ignores every supported sensitive type")
}

func TestLoadProviderRejectsAllSupportedTypesIgnoredUnderTokenDefault(t *testing.T) {
	assertInvalidPolicy(t, `{
		"default_operator": "token",
		"types": {
			"SECRET": {"operator": "ignore"},
			"EMAIL": {"operator": "ignore"},
			"PHONE": {"operator": "ignore"},
			"IPV4": {"operator": "ignore"},
			"IPV6": {"operator": "ignore"},
			"CARD": {"operator": "ignore"},
			"ACCT": {"operator": "ignore"},
			"URL": {"operator": "ignore"},
			"DATE": {"operator": "ignore"},
			"PERSON": {"operator": "ignore"},
			"ADDR": {"operator": "ignore"}
		}
	}`, "ignores every supported sensitive type")
}

func TestLoadProviderAllowsDefaultIgnoreWithExplicitMaskingCoverage(t *testing.T) {
	path := writePolicy(t, `{
		"default_operator": "ignore",
		"types": {
			"SECRET": {"operator": "block"}
		}
	}`)
	provider, _, err := LoadProvider(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	policy, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	if policy.DefaultOperator != types.OperatorIgnore {
		t.Fatalf("DefaultOperator = %q", policy.DefaultOperator)
	}
	if got := policy.Types[types.TypeSecret].Operator; got != types.OperatorBlock {
		t.Fatalf("SECRET operator = %q", got)
	}
}

func TestLoadProviderPathPrecedence(t *testing.T) {
	flagPath := writePolicy(t, `{"types":{"EMAIL":{"operator":"ignore"}}}`)
	envPath := writePolicy(t, `{"types":{"EMAIL":{"operator":"block"}}}`)
	t.Setenv("OPENCLOAK_POLICY", envPath)

	provider, source, err := LoadProvider(LoadOptions{Path: flagPath})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	if source.Path != flagPath || source.From != "flag" {
		t.Fatalf("source = %+v, want flag path", source)
	}
	policy, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	if got := policy.Types[types.TypeEmail].Operator; got != types.OperatorIgnore {
		t.Fatalf("EMAIL operator = %q, want ignore from flag path", got)
	}
}

func TestLoadProviderUsesEnvWhenFlagMissing(t *testing.T) {
	envPath := writePolicy(t, `{"types":{"SECRET":{"operator":"block"}}}`)
	t.Setenv("OPENCLOAK_POLICY", envPath)

	provider, source, err := LoadProvider(LoadOptions{})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	if source.Path != envPath || source.From != "env:OPENCLOAK_POLICY" {
		t.Fatalf("source = %+v, want env path", source)
	}
	policy, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	if got := policy.Types[types.TypeSecret].Operator; got != types.OperatorBlock {
		t.Fatalf("SECRET operator = %q, want block from env path", got)
	}
}

func TestLoadProviderUsesDefaultPathWhenPresent(t *testing.T) {
	home := t.TempDir()
	defaultDir := filepath.Join(home, ".opencloak")
	if err := os.MkdirAll(defaultDir, 0o700); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(defaultDir, "policy.json")
	if err := os.WriteFile(defaultPath, []byte(`{"types":{"EMAIL":{"operator":"ignore"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	provider, source, err := LoadProvider(LoadOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	if source.Path != defaultPath || source.From != "default" {
		t.Fatalf("source = %+v, want default path", source)
	}
	policy, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	if got := policy.Types[types.TypeEmail].Operator; got != types.OperatorIgnore {
		t.Fatalf("EMAIL operator = %q, want ignore", got)
	}
}

func TestLoadProviderNoDefaultFileReturnsNilProvider(t *testing.T) {
	provider, source, err := LoadProvider(LoadOptions{HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	if provider != nil || source.Loaded {
		t.Fatalf("provider/source = %v/%+v, want nil unloaded", provider, source)
	}
}

func TestLoadProviderExplicitMissingPathFails(t *testing.T) {
	_, _, err := LoadProvider(LoadOptions{Path: filepath.Join(t.TempDir(), "missing.json")})
	if err == nil {
		t.Fatal("LoadProvider returned nil error for missing explicit path")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProviderRejectsUnknownTopLevelKey(t *testing.T) {
	assertInvalidPolicy(t, `{
		"default_operator": "token",
		"label": "sk-live-should-not-be-accepted"
	}`, "unknown field")
}

func TestLoadProviderRejectsSecretLookingCommentKey(t *testing.T) {
	assertInvalidPolicy(t, `{
		"comment": "OPENAI_API_KEY=sk-live-should-not-be-accepted"
	}`, "unknown field")
}

func TestLoadProviderRejectsNestedMetadata(t *testing.T) {
	assertInvalidPolicy(t, `{
		"types": {
			"EMAIL": {
				"operator": "ignore",
				"metadata": {"owner_email": "admin@example.com", "raw": "/Users/me/.env"}
			}
		}
	}`, "unknown field")
}

func TestLoadProviderRejectsUnknownNestedKey(t *testing.T) {
	assertInvalidPolicy(t, `{
		"types": {
			"SECRET": {
				"operator": "block",
				"provider_label": "anthropic-api-key"
			}
		}
	}`, "unknown field")
}

func TestLoadProviderRejectsBadOperator(t *testing.T) {
	assertInvalidPolicy(t, `{"types":{"EMAIL":{"operator":"drop"}}}`, "unsupported operator")
}

func TestLoadProviderRejectsReservedOperators(t *testing.T) {
	cases := []string{"redact", "format_preserving"}
	for _, op := range cases {
		t.Run(op, func(t *testing.T) {
			assertInvalidPolicy(t, `{"types":{"SECRET":{"operator":"`+op+`"}}}`, "reserved")
		})
	}
}

func TestLoadProviderRejectsUnsupportedRuleSet(t *testing.T) {
	assertInvalidPolicy(t, `{"rule_sets":["strict-secrets"]}`, "rule_sets")
}

func TestLoadProviderRejectsUnknownType(t *testing.T) {
	assertInvalidPolicy(t, `{"types":{"CUSTOMER_ID":{"operator":"block"}}}`, "unsupported sensitive type")
}

func TestLoadProviderRejectsNonObjectJSON(t *testing.T) {
	assertInvalidPolicy(t, `null`, "JSON object")
}

func TestLoadedPolicyDrivesEngineMasking(t *testing.T) {
	path := writePolicy(t, `{
		"default_operator": "token",
		"types": {
			"EMAIL": {"operator": "ignore"}
		}
	}`)
	provider, _, err := LoadProvider(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	engine := newTestEngine(t, provider)

	masked, _, err := engine.Mask(context.Background(), opencloak.Scope{}, "contact user@example.com")
	if err != nil {
		t.Fatalf("Mask returned error: %v", err)
	}
	if !strings.Contains(masked, "user@example.com") {
		t.Fatalf("EMAIL should be ignored by loaded policy; got %q", masked)
	}

	masked, _, err = engine.Mask(context.Background(), opencloak.Scope{}, "key=AKIAIOSFODNN7EXAMPLE")
	if err != nil {
		t.Fatalf("Mask returned error for default-token SECRET: %v", err)
	}
	if strings.Contains(masked, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("SECRET should use default token policy; got %q", masked)
	}
}

func TestLoadedBlockPolicyDrivesEngineFailure(t *testing.T) {
	path := writePolicy(t, `{
		"default_operator": "token",
		"types": {
			"SECRET": {"operator": "block"}
		}
	}`)
	provider, _, err := LoadProvider(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	engine := newTestEngine(t, provider)

	_, _, err = engine.Mask(context.Background(), opencloak.Scope{}, "key=AKIAIOSFODNN7EXAMPLE")
	if err == nil {
		t.Fatal("Mask returned nil error for block policy")
	}
	if !errors.Is(err, opencloak.ErrBlocked) {
		t.Fatalf("Mask error = %T %v, want ErrBlocked", err, err)
	}
}

func TestProviderReturnsDefensiveCopies(t *testing.T) {
	path := writePolicy(t, `{"types":{"EMAIL":{"operator":"ignore"}}}`)
	provider, _, err := LoadProvider(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("LoadProvider returned error: %v", err)
	}
	first, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	first.Types[types.TypeEmail] = types.TypePolicy{Operator: types.OperatorBlock}

	second, err := provider.Policy(context.Background(), types.Scope{})
	if err != nil {
		t.Fatalf("Policy returned error: %v", err)
	}
	if got := second.Types[types.TypeEmail].Operator; got != types.OperatorIgnore {
		t.Fatalf("provider policy was mutated through returned map: got %q", got)
	}
}

func assertInvalidPolicy(t *testing.T, body, want string) {
	t.Helper()
	path := writePolicy(t, body)
	_, _, err := LoadProvider(LoadOptions{Path: path})
	if err == nil {
		t.Fatalf("LoadProvider(%s) returned nil error", body)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestEngine(t *testing.T, provider *Provider) *opencloak.Engine {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	keyPath := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	engine, err := opencloak.New(opencloak.Config{
		KeyPath: keyPath,
		Policy:  provider,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return engine
}
