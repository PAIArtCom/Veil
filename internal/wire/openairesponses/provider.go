// Package openairesponses walks the OpenAI Responses API shape used by Codex
// CLI: top-level instructions, input message text, function_call_output output,
// and agentic call argument fields.
package openairesponses

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/PAIArtCom/Veil/internal/wire"
)

const responsesOp = "responses"

// provider implements wire.Provider for OpenAI Responses.
type provider struct{}

// New returns a Provider for OpenAI Responses.
func New() wire.Provider {
	return &provider{}
}

func validateResponsesOp(op string) error {
	if op != responsesOp {
		return fmt.Errorf("openai-responses: unsupported op %q", op)
	}
	return nil
}

func validateJSON(kind string, body []byte) error {
	if !gjson.ValidBytes(body) {
		return fmt.Errorf("openai-responses: invalid %s JSON", kind)
	}
	return nil
}

// ExtractRequest extracts text that can carry user or local-tool data from an
// OpenAI Responses request. Static tools definitions and provider metadata are
// intentionally skipped.
func (p *provider) ExtractRequest(op string, body []byte) ([]wire.TextSpan, error) {
	if err := validateResponsesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("request", body); err != nil {
		return nil, err
	}

	var spans []wire.TextSpan
	if err := appendOptionalStringSpan(gjson.GetBytes(body, "instructions"), "instructions", "system", &spans); err != nil {
		return nil, err
	}
	if err := extractPromptVariables(gjson.GetBytes(body, "prompt"), &spans); err != nil {
		return nil, err
	}

	input := gjson.GetBytes(body, "input")
	switch input.Type {
	case gjson.String:
		if err := appendOptionalStringSpan(input, "input", "user", &spans); err != nil {
			return nil, err
		}
	case gjson.JSON:
		if !input.IsArray() {
			return nil, fmt.Errorf("openai-responses: unsupported input shape %q", input.Type.String())
		}
		var walkErr error
		input.ForEach(func(k, item gjson.Result) bool {
			walkErr = extractInputItem(item, fmt.Sprintf("input.%d", k.Int()), &spans)
			return walkErr == nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	case gjson.Null:
		// No input field: let the upstream API validate its own required fields.
	default:
		return nil, fmt.Errorf("openai-responses: unsupported input shape %q", input.Type.String())
	}
	return spans, nil
}

func extractPromptVariables(prompt gjson.Result, spans *[]wire.TextSpan) error {
	if !prompt.Exists() || prompt.Type == gjson.Null {
		return nil
	}
	if prompt.Type != gjson.JSON || !isJSONObject(prompt) {
		return fmt.Errorf("openai-responses: unsupported prompt shape")
	}
	variables := prompt.Get("variables")
	if !variables.Exists() || variables.Type == gjson.Null {
		return nil
	}
	if variables.Type != gjson.JSON || !isJSONObject(variables) {
		return fmt.Errorf("openai-responses: unsupported prompt.variables shape")
	}
	var walkErr error
	variables.ForEach(func(k, val gjson.Result) bool {
		path := "prompt.variables." + sjsonEscapeKey(k.Str)
		switch val.Type {
		case gjson.String:
			walkErr = appendOptionalStringSpan(val, path, "user", spans)
		case gjson.Null:
		default:
			walkErr = fmt.Errorf("openai-responses: unsupported prompt variable value at %s", path)
		}
		return walkErr == nil
	})
	return walkErr
}

func extractInputItem(item gjson.Result, path string, spans *[]wire.TextSpan) error {
	itemType := item.Get("type").Str
	if itemType == "" && item.Get("content").Exists() {
		itemType = "message"
	}
	role := item.Get("role").Str
	switch itemType {
	case "message":
		return extractMessageContent(item.Get("content"), path+".content", role, spans)
	case "function_call_output":
		return appendOptionalStringSpan(item.Get("output"), path+".output", role, spans)
	case "function_call":
		return appendOptionalStringSpan(item.Get("arguments"), path+".arguments", role, spans)
	case "mcp_call":
		return appendOptionalStringSpan(item.Get("arguments"), path+".arguments", role, spans)
	case "custom_tool_call":
		return appendOptionalStringSpan(item.Get("input"), path+".input", role, spans)
	case "code_interpreter_call":
		return appendOptionalStringSpan(item.Get("code"), path+".code", role, spans)
	case "reasoning":
		// Provider-origin reasoning summaries are not local tool/user payload.
	default:
		return fmt.Errorf("openai-responses: unsupported input item type %q at %s", itemType, path)
	}
	return nil
}

func extractMessageContent(content gjson.Result, path, role string, spans *[]wire.TextSpan) error {
	switch content.Type {
	case gjson.String:
		return appendOptionalStringSpan(content, path, role, spans)
	case gjson.JSON:
		if !content.IsArray() {
			return fmt.Errorf("openai-responses: unsupported message content shape at %s", path)
		}
		var walkErr error
		content.ForEach(func(k, block gjson.Result) bool {
			blockPath := fmt.Sprintf("%s.%d", path, k.Int())
			switch block.Get("type").Str {
			case "input_text", "output_text", "text":
				walkErr = appendOptionalStringSpan(block.Get("text"), blockPath+".text", role, spans)
			case "refusal":
				walkErr = appendOptionalStringSpan(block.Get("refusal"), blockPath+".refusal", role, spans)
			case "input_image", "input_file":
				walkErr = fmt.Errorf("openai-responses: unsupported message content block type %q at %s", block.Get("type").Str, blockPath)
				return false
			default:
				walkErr = fmt.Errorf("openai-responses: unsupported message content block type %q at %s", block.Get("type").Str, blockPath)
				return false
			}
			return walkErr == nil
		})
		return walkErr
	case gjson.Null:
	default:
		return fmt.Errorf("openai-responses: unsupported message content shape at %s", path)
	}
	return nil
}

func appendOptionalStringSpan(val gjson.Result, path, role string, spans *[]wire.TextSpan) error {
	if !val.Exists() || val.Type == gjson.Null {
		return nil
	}
	if val.Type != gjson.String {
		return fmt.Errorf("openai-responses: unsupported non-string value at %s", path)
	}
	if val.Str != "" {
		*spans = append(*spans, textSpanFromString(val, path, role))
	}
	return nil
}

func textSpanFromString(val gjson.Result, path, role string) wire.TextSpan {
	span := wire.TextSpan{Path: path, Text: val.Str, Role: role}
	if val.Type == gjson.String && strings.HasPrefix(val.Raw, `"`) {
		span.Start = val.Index
		span.End = val.Index + len(val.Raw)
	}
	return span
}

func isJSONObject(val gjson.Result) bool {
	raw := strings.TrimSpace(val.Raw)
	return strings.HasPrefix(raw, "{")
}

// ApplyRequest sets masked spans back into the request JSON.
func (p *provider) ApplyRequest(op string, body []byte, spans []wire.MaskedSpan) ([]byte, error) {
	if err := validateResponsesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("request", body); err != nil {
		return nil, err
	}

	if out, ok, err := wire.ApplyMaskedSpansByRange(body, spans); err != nil {
		return nil, fmt.Errorf("openai-responses: batch apply: %w", err)
	} else if ok {
		return out, nil
	}

	var err error
	for _, sp := range spans {
		body, err = sjson.SetBytes(body, sp.Path, sp.MaskedText)
		if err != nil {
			return nil, fmt.Errorf("openai-responses: apply span at %q: %w", sp.Path, err)
		}
	}
	return body, nil
}

// RestoreResponse restores tokens in non-streaming Responses bodies.
func (p *provider) RestoreResponse(op string, body []byte, restore wire.RestoreFunc) ([]byte, error) {
	if err := validateResponsesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("response", body); err != nil {
		return nil, err
	}

	var err error
	if gjson.GetBytes(body, "output").Exists() {
		body, err = restoreOutputArray(body, "output", restore)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

// RestoreSSEEvent restores tokens in one parsed Responses SSE event payload.
func (p *provider) RestoreSSEEvent(op string, eventData []byte, restore wire.RestoreFunc) ([]byte, error) {
	if err := validateResponsesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("SSE event", eventData); err != nil {
		return nil, err
	}

	switch gjson.GetBytes(eventData, "type").Str {
	case "response.output_text.delta", "response.refusal.delta", "response.function_call_arguments.delta",
		"response.mcp_call_arguments.delta", "response.custom_tool_call_input.delta",
		"response.code_interpreter_call_code.delta":
		return restoreStringAt(eventData, "delta", restore)
	case "response.output_text.done":
		return restoreStringAt(eventData, "text", restore)
	case "response.refusal.done":
		return restoreStringAt(eventData, "refusal", restore)
	case "response.function_call_arguments.done", "response.mcp_call_arguments.done":
		return restoreStringAt(eventData, "arguments", restore)
	case "response.custom_tool_call_input.done":
		return restoreStringAt(eventData, "input", restore)
	case "response.code_interpreter_call_code.done":
		return restoreStringAt(eventData, "code", restore)
	case "response.output_item.done":
		return restoreOutputItemAt(eventData, "item", restore)
	case "response.completed":
		if gjson.GetBytes(eventData, "response.output").Exists() {
			return restoreOutputArray(eventData, "response.output", restore)
		}
	}
	return eventData, nil
}

func restoreOutputArray(body []byte, path string, restore wire.RestoreFunc) ([]byte, error) {
	output := gjson.GetBytes(body, path)
	var err error
	output.ForEach(func(k, _ gjson.Result) bool {
		body, err = restoreOutputItemAt(body, fmt.Sprintf("%s.%d", path, k.Int()), restore)
		return err == nil
	})
	return body, err
}

func restoreOutputItemAt(body []byte, path string, restore wire.RestoreFunc) ([]byte, error) {
	item := gjson.GetBytes(body, path)
	switch item.Get("type").Str {
	case "message":
		return restoreMessageOutput(body, path+".content", restore)
	case "function_call":
		return restoreStringAt(body, path+".arguments", restore)
	case "mcp_call":
		return restoreStringAt(body, path+".arguments", restore)
	case "custom_tool_call":
		return restoreStringAt(body, path+".input", restore)
	case "code_interpreter_call":
		return restoreStringAt(body, path+".code", restore)
	case "function_call_output":
		return restoreStringAt(body, path+".output", restore)
	case "reasoning", "":
		return body, nil
	default:
		return body, nil
	}
}

func restoreMessageOutput(body []byte, path string, restore wire.RestoreFunc) ([]byte, error) {
	content := gjson.GetBytes(body, path)
	switch content.Type {
	case gjson.String:
		return restoreStringAt(body, path, restore)
	case gjson.JSON:
		var err error
		content.ForEach(func(k, block gjson.Result) bool {
			blockPath := fmt.Sprintf("%s.%d", path, k.Int())
			switch block.Get("type").Str {
			case "output_text", "input_text", "text":
				body, err = restoreStringAt(body, blockPath+".text", restore)
			case "refusal":
				body, err = restoreStringAt(body, blockPath+".refusal", restore)
			}
			return err == nil
		})
		return body, err
	}
	return body, nil
}

func restoreStringAt(body []byte, path string, restore wire.RestoreFunc) ([]byte, error) {
	val := gjson.GetBytes(body, path)
	if val.Type != gjson.String || val.Str == "" {
		return body, nil
	}
	restored, err := restore(val.Str)
	if err != nil {
		return nil, err
	}
	if restored == val.Str {
		return body, nil
	}
	out, err := sjson.SetBytes(body, path, restored)
	if err != nil {
		return nil, fmt.Errorf("openai-responses: set %s: %w", path, err)
	}
	return out, nil
}

func sjsonEscapeKey(key string) string {
	key = strings.ReplaceAll(key, `\`, `\\`)
	key = strings.ReplaceAll(key, ".", `\.`)
	key = strings.ReplaceAll(key, "|", `\|`)
	key = strings.ReplaceAll(key, ":", `\:`)
	return key
}
