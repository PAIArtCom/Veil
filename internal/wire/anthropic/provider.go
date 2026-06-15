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
// Blocks of type image/document/thinking are skipped.
func (p *provider) ExtractRequest(op string, body []byte) ([]wire.TextSpan, error) {
	var spans []wire.TextSpan

	// --- system field ---
	sys := gjson.GetBytes(body, "system")
	switch sys.Type {
	case gjson.String:
		if sys.Str != "" {
			spans = append(spans, wire.TextSpan{Path: "system", Text: sys.Str, Role: "system"})
		}
	case gjson.JSON:
		// array of content blocks
		sys.ForEach(func(key, val gjson.Result) bool {
			idx := key.Int()
			if val.Get("type").Str == "text" {
				text := val.Get("text").Str
				if text != "" {
					spans = append(spans, wire.TextSpan{
						Path: fmt.Sprintf("system.%d.text", idx),
						Text: text,
						Role: "system",
					})
				}
			}
			return true
		})
	}

	// --- messages[] ---
	msgs := gjson.GetBytes(body, "messages")
	msgs.ForEach(func(msgKey, msg gjson.Result) bool {
		mi := msgKey.Int()
		role := msg.Get("role").Str
		content := msg.Get("content")

		switch content.Type {
		case gjson.String:
			if content.Str != "" {
				spans = append(spans, wire.TextSpan{
					Path: fmt.Sprintf("messages.%d.content", mi),
					Text: content.Str,
					Role: role,
				})
			}
		case gjson.JSON:
			extractContentBlocks(content, mi, role, &spans)
		}
		return true
	})

	return spans, nil
}

// extractContentBlocks walks a messages[N].content array and appends spans for
// text, tool_use, and tool_result blocks. image/document/thinking blocks are
// skipped.
func extractContentBlocks(content gjson.Result, mi int64, role string, spans *[]wire.TextSpan) {
	content.ForEach(func(blkKey, blk gjson.Result) bool {
		bi := blkKey.Int()
		blkType := blk.Get("type").Str

		switch blkType {
		case "text":
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
			extractToolResultContent(innerContent, basePath, role, spans)

		default:
			// image, document, thinking: skip.
		}
		return true
	})
}

// extractToolResultContent handles the content field of a tool_result block:
// either a string or an array of text blocks.
func extractToolResultContent(innerContent gjson.Result, basePath, role string, spans *[]wire.TextSpan) {
	switch innerContent.Type {
	case gjson.String:
		if innerContent.Str != "" {
			*spans = append(*spans, wire.TextSpan{
				Path: basePath,
				Text: innerContent.Str,
				Role: role,
			})
		}
	case gjson.JSON:
		innerContent.ForEach(func(ik, ib gjson.Result) bool {
			ii := ik.Int()
			if ib.Get("type").Str == "text" {
				text := ib.Get("text").Str
				if text != "" {
					*spans = append(*spans, wire.TextSpan{
						Path: fmt.Sprintf("%s.%d.text", basePath, ii),
						Text: text,
						Role: role,
					})
				}
			}
			return true
		})
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
// sjson uses '.' as a separator and ':' as array modifier. We escape them with
// a backslash. See https://github.com/tidwall/sjson#path-syntax.
func sjsonEscapeKey(key string) string {
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
