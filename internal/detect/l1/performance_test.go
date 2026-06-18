package l1

import (
	"context"
	"strings"
	"testing"

	"github.com/cloakia/opencloak/internal/types"
)

var benchmarkCtx = context.Background()

func BenchmarkDetectNoSecretLargeCodePayload(b *testing.B) {
	text := strings.Repeat(`package main

import "fmt"

type Config struct {
	OrderID string
	TraceID string
	Endpoint string
}

func main() {
	cfg := Config{
		OrderID: "550e8400-e29b-41d4-a716-446655440000",
		TraceID: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		Endpoint: "https://example.com/api/v1/resources",
	}
	fmt.Println(cfg)
}
`, 24)
	benchmarkDetect(b, text, false)
}

func BenchmarkDetectMixedCodingPayload(b *testing.B) {
	text := `{
		"model":"gpt-5.4",
		"instructions":"Use local values only.",
		"input":[
			{"role":"user","content":"email user@example.com and connect to postgres://admin:hunter2@db.example.com/prod"},
			{"role":"tool","content":"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\nAWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY\nAWS_SESSION_TOKEN=FwoGZXIvYXdzEJr//////////wEaDEaK1ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCD\nOPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz12345\nANTHROPIC_API_KEY=sk-ant-abcdefghijklmnopqrstuvwxyz123456\nGITLAB_TOKEN=glpat-abcdefghijklmnopqrstuvwx123456\nNPM_TOKEN=npm_0123456789abcdefghijklmnopqrstuvwxyz\nTWINE_PASSWORD=pypi-AgEIcHlwaS5vcmcCJDExMTExMTExMTEx"},
			{"role":"tool","content":"DefaultEndpointsProtocol=https;AccountName=devstore;AccountKey=abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/==;EndpointSuffix=core.windows.net"},
			{"role":"tool","content":"order_id=550e8400-e29b-41d4-a716-446655440000 checksum=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824 token=${SERVICE_TOKEN}"}
		],
		"tools":[{"name":"run","description":"static examples stay static"}]
	}`
	benchmarkDetect(b, text, true)
}

func BenchmarkDetectSecretHeavyEnvPayload(b *testing.B) {
	text := strings.Join([]string{
		"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
		"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"AWS_SESSION_TOKEN=FwoGZXIvYXdzEJr//////////wEaDEaK1ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCD",
		"OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz12345",
		"ANTHROPIC_API_KEY=sk-ant-abcdefghijklmnopqrstuvwxyz123456",
		"GITLAB_TOKEN=glpat-abcdefghijklmnopqrstuvwx123456",
		"NPM_TOKEN=npm_0123456789abcdefghijklmnopqrstuvwxyz",
		"TWINE_PASSWORD=pypi-AgEIcHlwaS5vcmcCJDExMTExMTExMTEx",
		"SLACK_BOT_TOKEN=xoxb-123456789012-ABCDEFGHIJKL",
		"STRIPE_SECRET_KEY=sk_live_1234567890abcdefghijklmnop",
		"GOOGLE_API_KEY=AIzaSyD-1234567890abcdefghijklmnoQRSTUV",
		"AZURE_STORAGE_CONNECTION_STRING=DefaultEndpointsProtocol=https;AccountName=devstore;AccountKey=abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/==;EndpointSuffix=core.windows.net",
	}, "\n")
	benchmarkDetect(b, text, true)
}

func benchmarkDetect(b *testing.B, text string, wantFindings bool) {
	b.Helper()
	d := New()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		findings, err := d.Detect(benchmarkCtx, text)
		if err != nil {
			b.Fatal(err)
		}
		if wantFindings && !hasBenchmarkSecret(findings) {
			b.Fatal("expected SECRET findings")
		}
		if !wantFindings && hasBenchmarkSecret(findings) {
			b.Fatalf("unexpected SECRET findings in no-secret benchmark payload: %+v", findings)
		}
	}
}

func hasBenchmarkSecret(findings []types.Finding) bool {
	for _, f := range findings {
		if f.Type == types.TypeSecret {
			return true
		}
	}
	return false
}
