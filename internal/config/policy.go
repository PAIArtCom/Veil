package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cloakia/opencloak/internal/types"
)

const (
	// EnvPolicyPath names the environment variable used when --policy is not set.
	EnvPolicyPath = "OPENCLOAK_POLICY"
	// DefaultPolicyRelPath is resolved relative to the user's home directory.
	DefaultPolicyRelPath = ".opencloak/policy.json"
)

// LoadOptions controls local policy file resolution.
type LoadOptions struct {
	// Path is the explicit CLI flag value. It has highest precedence.
	Path string
	// EnvVar overrides EnvPolicyPath for tests and embedders.
	EnvVar string
	// HomeDir overrides os.UserHomeDir for tests and embedders.
	HomeDir string
}

// Source describes how a policy file was resolved.
type Source struct {
	Path   string
	From   string
	Loaded bool
}

// Provider is a static local-file PolicyProvider. It intentionally ignores scope:
// v0.1.0 OSS policy is single-user and local.
type Provider struct {
	policy types.Policy
}

// Policy returns a defensive copy of the loaded local policy.
func (p *Provider) Policy(_ context.Context, _ types.Scope) (types.Policy, error) {
	if p == nil {
		return types.Policy{}, errors.New("opencloak config: nil policy provider")
	}
	return clonePolicy(p.policy), nil
}

// LoadProvider resolves, reads, and validates a local policy file. If neither an
// explicit path nor an environment path is set and the default path does not
// exist, it returns nil provider and Loaded=false so callers can use the built-in
// engine policy.
func LoadProvider(opts LoadOptions) (*Provider, Source, error) {
	path, from, required, err := resolvePath(opts)
	if err != nil {
		return nil, Source{}, err
	}
	if path == "" {
		return nil, Source{Loaded: false}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if required && errors.Is(err, fs.ErrNotExist) {
			return nil, Source{}, fmt.Errorf("opencloak config: policy file from %s not found: %s", from, path)
		}
		return nil, Source{}, fmt.Errorf("opencloak config: read policy %s: %w", path, err)
	}

	policy, err := parsePolicy(data)
	if err != nil {
		return nil, Source{}, fmt.Errorf("opencloak config: invalid policy %s: %w", path, err)
	}
	return &Provider{policy: policy}, Source{Path: path, From: from, Loaded: true}, nil
}

func resolvePath(opts LoadOptions) (path, from string, required bool, err error) {
	if opts.Path != "" {
		return opts.Path, "flag", true, nil
	}

	envVar := opts.EnvVar
	if envVar == "" {
		envVar = EnvPolicyPath
	}
	if envPath := os.Getenv(envVar); envPath != "" {
		return envPath, "env:" + envVar, true, nil
	}

	home := opts.HomeDir
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return "", "", false, nil
		}
	}
	defaultPath := filepath.Join(home, DefaultPolicyRelPath)
	if _, err := os.Stat(defaultPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("opencloak config: inspect default policy %s: %w", defaultPath, err)
	}
	return defaultPath, "default", false, nil
}

type filePolicy struct {
	DefaultOperator *string               `json:"default_operator"`
	Types           map[string]typePolicy `json:"types"`
	RuleSets        []string              `json:"rule_sets"`
}

type typePolicy struct {
	Operator *string `json:"operator"`
}

func parsePolicy(data []byte) (types.Policy, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return types.Policy{}, errors.New("policy must be a JSON object")
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var file filePolicy
	if err := dec.Decode(&file); err != nil {
		return types.Policy{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return types.Policy{}, errors.New("multiple JSON values")
		}
		return types.Policy{}, err
	}
	if len(file.RuleSets) > 0 {
		return types.Policy{}, errors.New("rule_sets is reserved and must be empty")
	}

	policy := types.Policy{
		DefaultOperator: types.OperatorToken,
		Types:           make(map[types.Type]types.TypePolicy, len(file.Types)),
	}
	if file.DefaultOperator != nil {
		op, err := parseOperator("default_operator", *file.DefaultOperator)
		if err != nil {
			return types.Policy{}, err
		}
		policy.DefaultOperator = op
	}

	for name, tp := range file.Types {
		typ, err := parseType(name)
		if err != nil {
			return types.Policy{}, err
		}
		if tp.Operator == nil {
			return types.Policy{}, fmt.Errorf("types.%s.operator is required", name)
		}
		op, err := parseOperator("types."+name+".operator", *tp.Operator)
		if err != nil {
			return types.Policy{}, err
		}
		policy.Types[typ] = types.TypePolicy{Operator: op}
	}
	if len(policy.Types) == 0 {
		policy.Types = nil
	}
	return policy, nil
}

func parseOperator(field, raw string) (types.TransformOperator, error) {
	if raw == "" {
		return "", fmt.Errorf("%s must be one of token, ignore, block", field)
	}
	op := types.TransformOperator(raw)
	switch op {
	case types.OperatorToken, types.OperatorIgnore, types.OperatorBlock:
		return op, nil
	case types.OperatorRedact, types.OperatorFormatPreserving:
		return "", fmt.Errorf("%s operator %q is reserved in v0.1.0", field, raw)
	default:
		return "", fmt.Errorf("%s has unsupported operator %q", field, raw)
	}
}

func parseType(raw string) (types.Type, error) {
	typ := types.Type(raw)
	switch typ {
	case types.TypeSecret, types.TypeEmail, types.TypePhone, types.TypeIPv4, types.TypeIPv6,
		types.TypeCard, types.TypeAcct, types.TypeURL, types.TypeDate, types.TypePerson, types.TypeAddr:
		return typ, nil
	default:
		return "", fmt.Errorf("unsupported sensitive type %q", raw)
	}
}

func clonePolicy(policy types.Policy) types.Policy {
	out := types.Policy{
		DefaultOperator: policy.DefaultOperator,
	}
	if len(policy.Types) > 0 {
		out.Types = make(map[types.Type]types.TypePolicy, len(policy.Types))
		for typ, tp := range policy.Types {
			out.Types[typ] = tp
		}
	}
	if len(policy.RuleSets) > 0 {
		out.RuleSets = append([]string(nil), policy.RuleSets...)
	}
	return out
}
