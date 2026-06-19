// Package anthropic walks the Anthropic Messages API shape: system strings/blocks,
// messages[].content[].text, tool_use.input string leaves, tool_result.content,
// and content_block_delta SSE events.
package anthropic

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/cloakia/opencloak/internal/wire"
)

// provider implements wire.Provider for the Anthropic Messages API.
type provider struct{}

const messagesOp = "messages"

func validateMessagesOp(op string) error {
	if op != messagesOp {
		return fmt.Errorf("anthropic: unsupported op %q", op)
	}
	return nil
}

func validateJSON(kind string, body []byte) error {
	if !gjson.ValidBytes(body) {
		return fmt.Errorf("anthropic: invalid %s JSON", kind)
	}
	return nil
}

// New returns a Provider for the Anthropic Messages API.
func New() wire.Provider {
	return &provider{}
}

// ExtractRequest extracts all maskable text spans from an Anthropic
// /v1/messages request body. Paths follow gjson/sjson dot-index notation.
//
// Fields extracted:
//   - system: string form → path "system", text = the whole string
//   - system: array form → each block with type=="text" → path "system.N.text"
//   - messages[N].content: string form → path "messages.N.content"
//   - messages[N].content: block with type=="text" → path "messages.N.content.M.text"
//   - messages[N].content: block with type=="tool_use" → every string leaf inside
//     .input, recursively → path "messages.N.content.M.input.<key>"
//   - messages[N].content: block with type=="tool_result", content string →
//     path "messages.N.content.M.content"; content array → each type=="text"
//     block's .text → "messages.N.content.M.content.K.text"
//
// tools[] definitions are intentionally skipped (static schemas, not user data).
// Anthropic image/document payloads are opaque provider payloads and are not parsed,
// rewritten, or regenerated in v0.1.0. Thinking/control blocks preserve provider-native
// semantics and are not treated as user prompt text.
func (p *provider) ExtractRequest(op string, body []byte) ([]wire.TextSpan, error) {
	if err := validateMessagesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("request", body); err != nil {
		return nil, err
	}

	var spans []wire.TextSpan

	// --- system field ---
	sys := gjson.GetBytes(body, "system")
	switch sys.Type {
	case gjson.Null:
		// Optional field absent or null.
	case gjson.String:
		if sys.Str != "" {
			spans = append(spans, wire.TextSpan{Path: "system", Text: sys.Str, Role: "system"})
		}
	case gjson.JSON:
		if !sys.IsArray() {
			return nil, fmt.Errorf("anthropic: unsupported system shape")
		}
		// array of content blocks
		var walkErr error
		sys.ForEach(func(key, val gjson.Result) bool {
			idx := key.Int()
			if err := validateSystemBlock(val, "system", idx); err != nil {
				walkErr = err
				return false
			}
			switch val.Get("type").Str {
			case "text":
				if val.Get("text").Type != gjson.String {
					walkErr = fmt.Errorf("anthropic: system.%d text block has non-string text", idx)
					return false
				}
				text := val.Get("text").Str
				if text != "" {
					spans = append(spans, wire.TextSpan{
						Path: fmt.Sprintf("system.%d.text", idx),
						Text: text,
						Role: "system",
					})
				}
			case "image", "document", "thinking":
				// Opaque media/document payloads and provider thinking/control traces
				// are outside the v0.1.0 text/tool-I/O de-identification surface.
			}
			return true
		})
		if walkErr != nil {
			return nil, walkErr
		}
	default:
		return nil, fmt.Errorf("anthropic: unsupported system shape")
	}

	// --- messages[] ---
	msgs := gjson.GetBytes(body, "messages")
	if !msgs.Exists() {
		return nil, fmt.Errorf("anthropic: messages must be an array")
	}
	if msgs.Exists() && !msgs.IsArray() {
		return nil, fmt.Errorf("anthropic: messages must be an array")
	}
	var walkErr error
	msgs.ForEach(func(msgKey, msg gjson.Result) bool {
		mi := msgKey.Int()
		role := msg.Get("role").Str
		content := msg.Get("content")

		switch content.Type {
		case gjson.Null:
			walkErr = fmt.Errorf("anthropic: messages.%d missing content", mi)
			return false
		case gjson.String:
			if content.Str != "" {
				spans = append(spans, wire.TextSpan{
					Path: fmt.Sprintf("messages.%d.content", mi),
					Text: content.Str,
					Role: role,
				})
			}
		case gjson.JSON:
			if !content.IsArray() {
				walkErr = fmt.Errorf("anthropic: messages.%d content must be a string or array", mi)
				return false
			}
			if err := extractContentBlocks(content, mi, role, &spans); err != nil {
				walkErr = err
				return false
			}
		default:
			walkErr = fmt.Errorf("anthropic: messages.%d content must be a string or array", mi)
			return false
		}
		return true
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return spans, nil
}

// extractContentBlocks walks a messages[N].content array and appends spans for
// text, tool_use, and tool_result blocks. image/document payloads are opaque provider
// payloads and thinking/control traces are not user prompt text, so both remain outside
// the v0.1.0 replacement surface.
func extractContentBlocks(content gjson.Result, mi int64, role string, spans *[]wire.TextSpan) error {
	var walkErr error
	content.ForEach(func(blkKey, blk gjson.Result) bool {
		bi := blkKey.Int()
		if err := validateKnownBlock(blk, fmt.Sprintf("messages.%d.content", mi), bi); err != nil {
			walkErr = err
			return false
		}
		blkType := blk.Get("type").Str

		switch blkType {
		case "text":
			if blk.Get("text").Type != gjson.String {
				walkErr = fmt.Errorf("anthropic: messages.%d.content.%d text block has non-string text", mi, bi)
				return false
			}
			text := blk.Get("text").Str
			if text != "" {
				*spans = append(*spans, wire.TextSpan{
					Path: fmt.Sprintf("messages.%d.content.%d.text", mi, bi),
					Text: text,
					Role: role,
				})
			}

		case "tool_use":
			inputPath := fmt.Sprintf("messages.%d.content.%d.input", mi, bi)
			inputVal := blk.Get("input")
			extractStringLeaves(inputVal, inputPath, role, spans)

		case "tool_result":
			innerContent := blk.Get("content")
			basePath := fmt.Sprintf("messages.%d.content.%d.content", mi, bi)
			if err := extractToolResultContent(innerContent, basePath, role, spans); err != nil {
				walkErr = err
				return false
			}

		case "image", "document", "thinking":
			// Outside the v0.1.0 text/tool-I/O de-identification surface.
		}
		return true
	})
	return walkErr
}

// extractToolResultContent handles the content field of a tool_result block:
// either a string or an array of text blocks.
func extractToolResultContent(innerContent gjson.Result, basePath, role string, spans *[]wire.TextSpan) error {
	switch innerContent.Type {
	case gjson.Null:
		return fmt.Errorf("anthropic: %s missing content", basePath)
	case gjson.String:
		if innerContent.Str != "" {
			*spans = append(*spans, wire.TextSpan{
				Path: basePath,
				Text: innerContent.Str,
				Role: role,
			})
		}
	case gjson.JSON:
		if !innerContent.IsArray() {
			return fmt.Errorf("anthropic: %s must be a string or array", basePath)
		}
		var walkErr error
		innerContent.ForEach(func(ik, ib gjson.Result) bool {
			ii := ik.Int()
			if err := validateToolResultBlock(ib, basePath, ii); err != nil {
				walkErr = err
				return false
			}
			switch ib.Get("type").Str {
			case "text":
				if ib.Get("text").Type != gjson.String {
					walkErr = fmt.Errorf("anthropic: %s.%d text block has non-string text", basePath, ii)
					return false
				}
				text := ib.Get("text").Str
				if text != "" {
					*spans = append(*spans, wire.TextSpan{
						Path: fmt.Sprintf("%s.%d.text", basePath, ii),
						Text: text,
						Role: role,
					})
				}
			case "image", "document":
				// Opaque media/document payloads are outside the text replacement
				// surface; OpenCloak does not parse or regenerate them.
			}
			return true
		})
		if walkErr != nil {
			return walkErr
		}
	default:
		return fmt.Errorf("anthropic: %s must be a string or array", basePath)
	}
	return nil
}

func validateKnownBlock(block gjson.Result, parent string, index int64) error {
	if !block.IsObject() {
		return fmt.Errorf("anthropic: %s.%d must be an object block", parent, index)
	}
	typeVal := block.Get("type")
	if typeVal.Type != gjson.String || typeVal.Str == "" {
		return fmt.Errorf("anthropic: %s.%d missing string type", parent, index)
	}
	switch typeVal.Str {
	case "text", "tool_use", "tool_result", "image", "document", "thinking":
		return nil
	default:
		return fmt.Errorf("anthropic: unsupported block type %q at %s.%d", typeVal.Str, parent, index)
	}
}

func validateSystemBlock(block gjson.Result, parent string, index int64) error {
	if !block.IsObject() {
		return fmt.Errorf("anthropic: %s.%d must be an object block", parent, index)
	}
	typeVal := block.Get("type")
	if typeVal.Type != gjson.String || typeVal.Str == "" {
		return fmt.Errorf("anthropic: %s.%d missing string type", parent, index)
	}
	switch typeVal.Str {
	case "text", "image", "document", "thinking":
		return nil
	default:
		return fmt.Errorf("anthropic: unsupported system block type %q at %s.%d", typeVal.Str, parent, index)
	}
}

func validateToolResultBlock(block gjson.Result, parent string, index int64) error {
	if !block.IsObject() {
		return fmt.Errorf("anthropic: %s.%d must be an object block", parent, index)
	}
	typeVal := block.Get("type")
	if typeVal.Type != gjson.String || typeVal.Str == "" {
		return fmt.Errorf("anthropic: %s.%d missing string type", parent, index)
	}
	switch typeVal.Str {
	case "text", "image", "document":
		return nil
	default:
		return fmt.Errorf("anthropic: unsupported tool_result block type %q at %s.%d", typeVal.Str, parent, index)
	}
}

// extractStringLeaves recursively traverses a gjson.Result (object or array)
// and appends a TextSpan for every string leaf.
func extractStringLeaves(val gjson.Result, path string, role string, spans *[]wire.TextSpan) {
	switch val.Type {
	case gjson.String:
		if val.Str != "" {
			*spans = append(*spans, wire.TextSpan{Path: path, Text: val.Str, Role: role})
		}
	case gjson.JSON:
		val.ForEach(func(k, v gjson.Result) bool {
			var childPath string
			if k.Type == gjson.String {
				// Object key — escape dots and at-signs so sjson treats it as a literal key.
				childPath = path + "." + sjsonEscapeKey(k.Str)
			} else {
				// Array index.
				childPath = fmt.Sprintf("%s.%d", path, k.Int())
			}
			extractStringLeaves(v, childPath, role, spans)
			return true
		})
	}
	// Numbers, booleans, null → not string, skip.
}

// sjsonEscapeKey escapes special characters in an object key so sjson treats
// it as a literal path component rather than a nested path.
// sjson uses '\' as an escape marker, '.' as a separator, and ':' as array
// modifier. We escape them with a backslash. See
// https://github.com/tidwall/sjson#path-syntax.
func sjsonEscapeKey(key string) string {
	key = strings.ReplaceAll(key, `\`, `\\`)
	key = strings.ReplaceAll(key, ".", `\.`)
	key = strings.ReplaceAll(key, "|", `\|`)
	key = strings.ReplaceAll(key, ":", `\:`)
	return key
}

// ApplyRequest sets each masked span back into the body using sjson surgical
// set — only the targeted string values change; all other bytes are preserved.
// Spans are applied sequentially; because sjson paths are structural (key/index
// based, not byte-offset based) they remain valid regardless of value-length
// changes from earlier sets.
func (p *provider) ApplyRequest(op string, body []byte, spans []wire.MaskedSpan) ([]byte, error) {
	if err := validateMessagesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("request", body); err != nil {
		return nil, err
	}

	var err error
	for _, sp := range spans {
		body, err = sjson.SetBytes(body, sp.Path, sp.MaskedText)
		if err != nil {
			return nil, fmt.Errorf("anthropic: apply span at %q: %w", sp.Path, err)
		}
	}
	return body, nil
}

// RestoreResponse restores tokens in a non-streaming Anthropic response body.
// It walks content blocks of the assistant message:
//   - type=="text" → restore .text
//   - type=="tool_use" → restore every string leaf in .input recursively
//
// All other block types are left untouched. The body shape is preserved
// byte-for-byte except for the restored string values.
func (p *provider) RestoreResponse(op string, body []byte, restore wire.RestoreFunc) ([]byte, error) {
	if err := validateMessagesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("response", body); err != nil {
		return nil, err
	}

	content := gjson.GetBytes(body, "content")
	if !content.Exists() {
		return body, nil
	}

	var restoreErr error
	content.ForEach(func(key, blk gjson.Result) bool {
		idx := key.Int()
		blkType := blk.Get("type").Str

		switch blkType {
		case "text":
			text := blk.Get("text").Str
			restored, err := restore(text)
			if err != nil {
				restoreErr = err
				return false
			}
			if restored != text {
				path := fmt.Sprintf("content.%d.text", idx)
				var setErr error
				body, setErr = sjson.SetBytes(body, path, restored)
				if setErr != nil {
					restoreErr = setErr
					return false
				}
			}

		case "tool_use":
			inputVal := blk.Get("input")
			path := fmt.Sprintf("content.%d.input", idx)
			newBody, setErr := restoreStringLeavesInto(body, inputVal, path, restore)
			if setErr != nil {
				restoreErr = setErr
				return false
			}
			body = newBody
		}
		return true
	})

	return body, restoreErr
}

// RestoreSSEEvent restores tokens in a single parsed Anthropic SSE event
// payload. Only content_block_delta events are touched:
//   - delta.type=="text_delta" → restore delta.text
//   - delta.type=="input_json_delta" → restore delta.partial_json
//
// Any other event type is returned unchanged.
func (p *provider) RestoreSSEEvent(op string, eventData []byte, restore wire.RestoreFunc) ([]byte, error) {
	if err := validateMessagesOp(op); err != nil {
		return nil, err
	}
	if err := validateJSON("SSE event", eventData); err != nil {
		return nil, err
	}

	evType := gjson.GetBytes(eventData, "type").Str
	if evType != "content_block_delta" {
		return eventData, nil
	}

	deltaType := gjson.GetBytes(eventData, "delta.type").Str
	switch deltaType {
	case "text_delta":
		text := gjson.GetBytes(eventData, "delta.text").Str
		restored, err := restore(text)
		if err != nil {
			return nil, err
		}
		if restored == text {
			return eventData, nil
		}
		out, err := sjson.SetBytes(eventData, "delta.text", restored)
		if err != nil {
			return nil, fmt.Errorf("anthropic: set delta.text: %w", err)
		}
		return out, nil

	case "input_json_delta":
		text := gjson.GetBytes(eventData, "delta.partial_json").Str
		restored, err := restore(text)
		if err != nil {
			return nil, err
		}
		if restored == text {
			return eventData, nil
		}
		out, err := sjson.SetBytes(eventData, "delta.partial_json", restored)
		if err != nil {
			return nil, fmt.Errorf("anthropic: set delta.partial_json: %w", err)
		}
		return out, nil
	}

	return eventData, nil
}

// restoreStringLeavesInto walks val (a gjson.Result rooted at basePath in
// body) and restores every string leaf in-place via sjson.SetBytes. It returns
// the updated body.
func restoreStringLeavesInto(body []byte, val gjson.Result, path string, restore wire.RestoreFunc) ([]byte, error) {
	switch val.Type {
	case gjson.String:
		restored, err := restore(val.Str)
		if err != nil {
			return nil, err
		}
		if restored == val.Str {
			return body, nil
		}
		body, err = sjson.SetBytes(body, path, restored)
		if err != nil {
			return nil, fmt.Errorf("anthropic: restore leaf at %q: %w", path, err)
		}
		return body, nil

	case gjson.JSON:
		var err error
		val.ForEach(func(k, v gjson.Result) bool {
			var childPath string
			if k.Type == gjson.String {
				childPath = path + "." + sjsonEscapeKey(k.Str)
			} else {
				childPath = fmt.Sprintf("%s.%d", path, k.Int())
			}
			body, err = restoreStringLeavesInto(body, v, childPath, restore)
			return err == nil
		})
		return body, err
	}
	return body, nil
}
