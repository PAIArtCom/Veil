package l1

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/PAIArtCom/Veil/internal/types"
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

func BenchmarkDetectTieredLongContextPayload(b *testing.B) {
	tiers := []struct {
		name    string
		records int
	}{
		{name: "tier1_interactive", records: 12},
		{name: "tier2_agent_turn", records: 96},
		{name: "tier3_long_context", records: 512},
		{name: "tier4_stress", records: 1024},
	}

	for _, tier := range tiers {
		text := benchmarkLongContextPayload(tier.records)
		b.Run(tier.name, func(b *testing.B) {
			benchmarkDetect(b, text, true)
		})
	}
}

func BenchmarkDetectComplexBusinessPayload(b *testing.B) {
	text := benchmarkComplexBusinessPayload(240)
	benchmarkDetect(b, text, true)
}

func benchmarkDetect(b *testing.B, text string, wantFindings bool) {
	b.Helper()
	d := New()
	b.SetBytes(int64(len(text)))
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

func benchmarkLongContextPayload(records int) string {
	var sb strings.Builder
	sb.Grow(records * 700)
	for i := 0; i < records; i++ {
		fmt.Fprintf(&sb, "### workspace note %05d\n", i)
		fmt.Fprintf(&sb, "component=checkout-worker trace_id=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824 order_id=550e8400-e29b-41d4-a716-446655440000\n")
		fmt.Fprintf(&sb, "code reference: process.env.API_KEY parseToken() /srv/app/.env.example ${SERVICE_TOKEN}\n")
		fmt.Fprintf(&sb, "customer_email=customer%05d@example.com phone=+1 (415) 555-%04d ip=203.0.113.%d ipv6=2001:db8::%x\n", i, i%10000, i%255, i%4096)
		if i%8 == 0 {
			fmt.Fprintf(&sb, "OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz1234567890%04d\n", i)
		}
		if i%13 == 0 {
			fmt.Fprintf(&sb, "payment_card=4242 4242 4242 4242 iban=GB82WEST12345698765432 callback=https://api.example.com/tenant/%05d?token=not_a_secret\n", i)
		}
		if i%21 == 0 {
			fmt.Fprintf(&sb, "tool_result: postgres://app:Sup3rSecretPass%05d@db.internal.example/prod migrated 27 rows\n", i)
		}
		sb.WriteString("notes: This block intentionally mixes source-like text, identifiers, URLs, and structured PII.\n\n")
	}
	return sb.String()
}

func benchmarkComplexBusinessPayload(records int) string {
	var sb strings.Builder
	sb.Grow(records * 900)
	sb.WriteString(`{"tenant":"acme-retail","workflow":"incident-response","items":[`)
	for i := 0; i < records; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"ticket":"OC-%06d","customer":"customer%06d@example.com","summary":"refund investigation for +1-212-555-%04d and https://billing.example.com/account/%06d",`, i, i, i%10000, i)
		fmt.Fprintf(&sb, `"tool_output":"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\nSLACK_BOT_TOKEN=xoxb-123456789012-ABCDEFGHIJKL\nSTRIPE_SECRET_KEY=sk_live_1234567890abcdefghijklmnop\norder_id=550e8400-e29b-41d4-a716-446655440000\nhash=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",`)
		fmt.Fprintf(&sb, `"developer_notes":"Do not mask process.env.API_KEY or parseToken(); mask literal GOOGLE_API_KEY=AIzaSyD-1234567890abcdefghijklmnoQRSTUV when it appears."}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}
