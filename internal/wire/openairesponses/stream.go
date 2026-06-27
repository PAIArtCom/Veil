package openairesponses

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/PAIArtCom/Veil/internal/mask"
	"github.com/PAIArtCom/Veil/internal/token"
	"github.com/PAIArtCom/Veil/internal/wire"
)

// NewStreamRestorer returns a fresh stateful SSE restorer for the OpenAI
// Responses stream. Unsupported ops fail closed before any bytes are relayed.
func (p *provider) NewStreamRestorer(op string) (wire.StreamRestorer, error) {
	if err := validateResponsesOp(op); err != nil {
		return nil, err
	}
	return &streamRestorer{
		text: make(map[string]*textState),
		args: make(map[string]*argState),
	}, nil
}

type textState struct {
	eventType string
	tail      []byte
}

type argState struct {
	doneType string
	delta    string
	buf      strings.Builder
}

type streamRestorer struct {
	text map[string]*textState
	args map[string]*argState
}

func (s *streamRestorer) Event(eventData []byte, restore wire.RestoreFunc) ([][]byte, error) {
	eventType := gjson.GetBytes(eventData, "type").Str
	switch eventType {
	case "response.output_text.delta", "response.refusal.delta":
		return s.handleTextDelta(eventData, restore)
	case "response.output_text.done", "response.refusal.done":
		return s.handleTextDone(eventData, restore)
	case "response.function_call_arguments.delta", "response.mcp_call_arguments.delta",
		"response.custom_tool_call_input.delta", "response.code_interpreter_call_code.delta":
		return s.handleArgDelta(eventData)
	case "response.function_call_arguments.done", "response.mcp_call_arguments.done",
		"response.custom_tool_call_input.done", "response.code_interpreter_call_code.done":
		return s.handleArgDone(eventData, restore)
	case "response.output_item.done", "response.completed":
		out, err := (&provider{}).RestoreSSEEvent(responsesOp, eventData, restore)
		if err != nil {
			return nil, err
		}
		return [][]byte{out}, nil
	default:
		return [][]byte{eventData}, nil
	}
}

func (s *streamRestorer) handleTextDelta(eventData []byte, restore wire.RestoreFunc) ([][]byte, error) {
	key := textKey(eventData)
	st := s.text[key]
	if st == nil {
		st = &textState{eventType: gjson.GetBytes(eventData, "type").Str}
		s.text[key] = st
	}
	combined := append(st.tail, []byte(gjson.GetBytes(eventData, "delta").Str)...)
	tokenDanger := token.PartialSuffixStart(combined)
	surrogateDanger := mask.PartialSurrogateSuffixStart(combined)
	danger := tokenDanger
	maxHold := token.MaxTokenLen
	if surrogateDanger < danger {
		danger = surrogateDanger
		maxHold = mask.MaxSurrogateLen
	}
	if len(combined)-danger > maxHold {
		danger = len(combined) - maxHold
	}
	safe := combined[:danger]
	st.tail = append([]byte(nil), combined[danger:]...)
	restoredSafe, err := restore(string(safe))
	if err != nil {
		return nil, err
	}
	if restoredSafe == "" {
		return [][]byte{}, nil
	}
	out, err := sjson.SetBytes(eventData, "delta", restoredSafe)
	if err != nil {
		return nil, fmt.Errorf("openai-responses: set delta: %w", err)
	}
	return [][]byte{out}, nil
}

func (s *streamRestorer) handleTextDone(eventData []byte, restore wire.RestoreFunc) ([][]byte, error) {
	key := textKey(eventData)
	var out [][]byte
	if st := s.text[key]; st != nil {
		delete(s.text, key)
		if len(st.tail) > 0 {
			restoredTail, err := restore(string(st.tail))
			if err != nil {
				return nil, err
			}
			if restoredTail != "" {
				synthetic, err := syntheticTextDelta(eventData, st.eventType, restoredTail)
				if err != nil {
					return nil, err
				}
				out = append(out, synthetic)
			}
		}
	}
	restored, err := (&provider{}).RestoreSSEEvent(responsesOp, eventData, restore)
	if err != nil {
		return nil, err
	}
	return append(out, restored), nil
}

func (s *streamRestorer) handleArgDelta(eventData []byte) ([][]byte, error) {
	key := argKey(eventData)
	st := s.args[key]
	if st == nil {
		eventType := gjson.GetBytes(eventData, "type").Str
		st = &argState{
			doneType: strings.TrimSuffix(eventType, ".delta") + ".done",
			delta:    eventType,
		}
		s.args[key] = st
	}
	st.buf.WriteString(gjson.GetBytes(eventData, "delta").Str)
	return [][]byte{}, nil
}

func (s *streamRestorer) handleArgDone(eventData []byte, restore wire.RestoreFunc) ([][]byte, error) {
	key := argKey(eventData)
	eventType := gjson.GetBytes(eventData, "type").Str
	st := s.args[key]
	if st == nil {
		st = &argState{
			doneType: eventType,
			delta:    strings.TrimSuffix(eventType, ".done") + ".delta",
		}
	}
	delete(s.args, key)

	field := doneValueField(eventType)
	complete := gjson.GetBytes(eventData, field).Str
	if complete == "" && st.buf.Len() > 0 {
		complete = st.buf.String()
	}
	restored, err := restore(complete)
	if err != nil {
		return nil, err
	}
	var out [][]byte
	if restored != "" {
		synthetic, err := syntheticArgDelta(eventData, st.delta, restored)
		if err != nil {
			return nil, err
		}
		out = append(out, synthetic)
	}
	if field != "" {
		eventData, err = sjson.SetBytes(eventData, field, restored)
		if err != nil {
			return nil, fmt.Errorf("openai-responses: set %s: %w", field, err)
		}
	}
	return append(out, eventData), nil
}

func (s *streamRestorer) Flush(restore wire.RestoreFunc) ([][]byte, error) {
	var out [][]byte
	textKeys := make([]string, 0, len(s.text))
	for k := range s.text {
		textKeys = append(textKeys, k)
	}
	sort.Strings(textKeys)
	for _, k := range textKeys {
		st := s.text[k]
		if len(st.tail) == 0 {
			continue
		}
		restored, err := restore(string(st.tail))
		if err != nil {
			return nil, err
		}
		if restored != "" {
			out = append(out, minimalDelta(st.eventType, restored))
		}
	}
	s.text = make(map[string]*textState)

	argKeys := make([]string, 0, len(s.args))
	for k := range s.args {
		argKeys = append(argKeys, k)
	}
	sort.Strings(argKeys)
	for _, k := range argKeys {
		st := s.args[k]
		if st.buf.Len() == 0 {
			continue
		}
		restored, err := restore(st.buf.String())
		if err != nil {
			return nil, err
		}
		out = append(out, minimalDelta(st.delta, restored))
	}
	s.args = make(map[string]*argState)
	return out, nil
}

func textKey(eventData []byte) string {
	return gjson.GetBytes(eventData, "item_id").Str + "|" +
		gjson.GetBytes(eventData, "output_index").String() + "|" +
		gjson.GetBytes(eventData, "content_index").String()
}

func argKey(eventData []byte) string {
	return gjson.GetBytes(eventData, "item_id").Str + "|" +
		gjson.GetBytes(eventData, "output_index").String()
}

func doneValueField(eventType string) string {
	switch eventType {
	case "response.output_text.done":
		return "text"
	case "response.refusal.done":
		return "refusal"
	case "response.function_call_arguments.done", "response.mcp_call_arguments.done":
		return "arguments"
	case "response.custom_tool_call_input.done":
		return "input"
	case "response.code_interpreter_call_code.done":
		return "code"
	default:
		return ""
	}
}

func syntheticTextDelta(template []byte, eventType, delta string) ([]byte, error) {
	out, err := sjson.SetBytes(template, "type", eventType)
	if err != nil {
		return nil, err
	}
	out, err = sjson.SetBytes(out, "delta", delta)
	if err != nil {
		return nil, err
	}
	for _, field := range []string{"text", "refusal", "arguments", "input", "code"} {
		out, _ = sjson.DeleteBytes(out, field)
	}
	return out, nil
}

func syntheticArgDelta(template []byte, eventType, delta string) ([]byte, error) {
	out, err := sjson.SetBytes(template, "type", eventType)
	if err != nil {
		return nil, err
	}
	out, err = sjson.SetBytes(out, "delta", delta)
	if err != nil {
		return nil, err
	}
	for _, field := range []string{"arguments", "input", "code"} {
		out, _ = sjson.DeleteBytes(out, field)
	}
	return out, nil
}

func minimalDelta(eventType, delta string) []byte {
	out, _ := sjson.SetBytes([]byte(`{}`), "type", eventType)
	out, _ = sjson.SetBytes(out, "delta", delta)
	return out
}
