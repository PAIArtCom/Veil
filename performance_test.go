package opencloak_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	opencloak "github.com/cloakia/opencloak"
)

func BenchmarkMaskTextTieredBusinessContext(b *testing.B) {
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
		text := benchmarkBusinessText(tier.records)
		b.Run(tier.name, func(b *testing.B) {
			e := newTestEngine(b)
			scope := opencloak.Scope{Tenant: "bench", Session: "mask-text", Project: tier.name}
			verifyMaskTextBenchmark(b, e, scope, text)
			b.SetBytes(int64(len(text)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := e.Mask(ctx, scope, text); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMaskRequestOpenAIResponsesBusinessContext(b *testing.B) {
	tiers := []struct {
		name    string
		records int
	}{
		{name: "tier1_interactive", records: 8},
		{name: "tier2_agent_turn", records: 48},
		{name: "tier3_long_context", records: 192},
	}

	for _, tier := range tiers {
		body := benchmarkResponsesBody(b, tier.records)
		b.Run(tier.name, func(b *testing.B) {
			e := newTestEngine(b)
			scope := opencloak.Scope{Tenant: "bench", Session: "responses", Project: tier.name}
			verifyMaskRequestBenchmark(b, e, scope, "openai-responses", "responses", body)
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := e.MaskRequest(ctx, scope, "openai-responses", "responses", body); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMaskRequestAnthropicBusinessTranscript(b *testing.B) {
	tiers := []struct {
		name    string
		records int
	}{
		{name: "tier1_interactive", records: 6},
		{name: "tier2_agent_turn", records: 36},
		{name: "tier3_long_context", records: 144},
	}

	for _, tier := range tiers {
		body := benchmarkAnthropicBody(b, tier.records)
		b.Run(tier.name, func(b *testing.B) {
			e := newTestEngine(b)
			scope := opencloak.Scope{Tenant: "bench", Session: "anthropic", Project: tier.name}
			verifyMaskRequestBenchmark(b, e, scope, "anthropic", "messages", body)
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := e.MaskRequest(ctx, scope, "anthropic", "messages", body); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func verifyMaskTextBenchmark(b *testing.B, e *opencloak.Engine, scope opencloak.Scope, text string) {
	b.Helper()
	masked, _, err := e.Mask(ctx, scope, text)
	if err != nil {
		b.Fatalf("Mask: %v", err)
	}
	for _, plain := range []string{
		"customer000000@example.com",
		"sk-proj-abcdefghijklmnopqrstuvwxyz1234567890",
		"postgres://app:Sup3rSecretPass000000@db.internal.example/prod",
	} {
		if strings.Contains(masked, plain) {
			b.Fatalf("benchmark plaintext %q leaked in masked text", plain)
		}
	}
	if !strings.Contains(masked, "OpenCloak_") {
		b.Fatalf("benchmark text produced no OpenCloak token")
	}
}

func verifyMaskRequestBenchmark(b *testing.B, e *opencloak.Engine, scope opencloak.Scope, provider, op string, body []byte) {
	b.Helper()
	masked, _, err := e.MaskRequest(ctx, scope, provider, op, body)
	if err != nil {
		b.Fatalf("MaskRequest(%s/%s): %v", provider, op, err)
	}
	for _, plain := range [][]byte{
		[]byte("customer000000@example.com"),
		[]byte("sk-proj-abcdefghijklmnopqrstuvwxyz1234567890"),
		[]byte("postgres://app:Sup3rSecretPass000000@db.internal.example/prod"),
	} {
		if bytes.Contains(masked, plain) {
			b.Fatalf("benchmark plaintext %q leaked in masked request", plain)
		}
	}
	if !bytes.Contains(masked, []byte("OpenCloak_")) {
		b.Fatalf("benchmark request produced no OpenCloak token")
	}
}

func benchmarkBusinessText(records int) string {
	var sb strings.Builder
	sb.Grow(records * 900)
	for i := 0; i < records; i++ {
		fmt.Fprintf(&sb, "case_id=OC-%06d tenant=acme-retail region=us-east\n", i)
		fmt.Fprintf(&sb, "customer=customer%06d@example.com phone=+1-212-555-%04d card=4242 4242 4242 4242 iban=GB82WEST12345698765432\n", i, i%10000)
		fmt.Fprintf(&sb, "service_url=https://billing.example.com/account/%06d callback=https://hooks.example.com/orders/%06d\n", i, i)
		if i%5 == 0 {
			fmt.Fprintf(&sb, "OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz1234567890\n")
		}
		if i%9 == 0 {
			fmt.Fprintf(&sb, "dsn=postgres://app:Sup3rSecretPass%06d@db.internal.example/prod\n", i)
		}
		if i%17 == 0 {
			fmt.Fprintf(&sb, "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE STRIPE_SECRET_KEY=sk_live_1234567890abcdefghijklmnop\n")
		}
		sb.WriteString("noise: process.env.API_KEY parseToken() ${SERVICE_TOKEN} /srv/app/.env.example 550e8400-e29b-41d4-a716-446655440000\n\n")
	}
	return sb.String()
}

func benchmarkResponsesBody(b *testing.B, records int) []byte {
	b.Helper()
	input := make([]map[string]any, 0, records*2)
	for i := 0; i < records; i++ {
		input = append(input, map[string]any{
			"type": "message",
			"role": "user",
			"content": []map[string]string{{
				"type": "input_text",
				"text": benchmarkBusinessText(1) +
					fmt.Sprintf("response_case=%06d contact=customer%06d@example.com", i, i),
			}},
		})
		if i%4 == 0 {
			args := benchmarkJSONArgString(b, map[string]string{
				"dsn":   fmt.Sprintf("postgres://app:Sup3rSecretPass%06d@db.internal.example/prod", i),
				"email": fmt.Sprintf("customer%06d@example.com", i),
			})
			input = append(input, map[string]any{
				"type":      "function_call",
				"call_id":   fmt.Sprintf("call_%06d", i),
				"name":      "run_migration",
				"arguments": args,
			})
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": fmt.Sprintf("call_%06d", i),
				"output":  "OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz1234567890\ncustomer=customer000000@example.com",
			})
		}
		if i%11 == 0 {
			input = append(input, map[string]any{
				"type":    "code_interpreter_call",
				"call_id": fmt.Sprintf("code_%06d", i),
				"code":    "print('customer000000@example.com')\nsecret='sk-proj-abcdefghijklmnopqrstuvwxyz1234567890'",
			})
		}
	}
	body := map[string]any{
		"model":        "gpt-5.4",
		"instructions": "Use only local credentials. OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz1234567890",
		"prompt": map[string]any{
			"id": "pmpt_benchmark",
			"variables": map[string]string{
				"contact": "customer000000@example.com",
				"dsn":     "postgres://app:Sup3rSecretPass000000@db.internal.example/prod",
			},
		},
		"input": input,
		"tools": []map[string]string{{
			"type":        "function",
			"name":        "run_migration",
			"description": "Static schema example uses ${OPENAI_API_KEY}, not a literal credential.",
		}},
		"stream": true,
	}
	out, err := json.Marshal(body)
	if err != nil {
		b.Fatalf("marshal responses benchmark body: %v", err)
	}
	return out
}

func benchmarkAnthropicBody(b *testing.B, records int) []byte {
	b.Helper()
	messages := make([]map[string]any, 0, records)
	for i := 0; i < records; i++ {
		messages = append(messages, map[string]any{
			"role": "user",
			"content": []map[string]any{
				{
					"type": "text",
					"text": benchmarkBusinessText(1) +
						fmt.Sprintf("anthropic_case=%06d email=customer%06d@example.com", i, i),
				},
				{
					"type": "tool_use",
					"id":   fmt.Sprintf("tu_%06d", i),
					"name": "run_migration",
					"input": map[string]string{
						"dsn":   fmt.Sprintf("postgres://app:Sup3rSecretPass%06d@db.internal.example/prod", i),
						"email": fmt.Sprintf("customer%06d@example.com", i),
					},
				},
				{
					"type":        "tool_result",
					"tool_use_id": fmt.Sprintf("tu_%06d", i),
					"content":     "OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz1234567890\ncustomer=customer000000@example.com",
				},
			},
		})
	}
	body := map[string]any{
		"model":      "claude-opus-4-5",
		"max_tokens": 1024,
		"system": []map[string]string{{
			"type": "text",
			"text": "Use local-only credential OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz1234567890",
		}},
		"messages": messages,
	}
	out, err := json.Marshal(body)
	if err != nil {
		b.Fatalf("marshal anthropic benchmark body: %v", err)
	}
	return out
}

func benchmarkJSONArgString(b *testing.B, value map[string]string) string {
	b.Helper()
	out, err := json.Marshal(value)
	if err != nil {
		b.Fatalf("marshal benchmark arguments: %v", err)
	}
	return string(out)
}
