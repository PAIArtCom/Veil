package anthropic

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/cloakia/opencloak/internal/token"
	"github.com/cloakia/opencloak/internal/wire"
)

// NewStreamRestorer returns a fresh stateful SSE restorer for the Anthropic
// Messages stream. Phase 0 supports only the Messages operation; unsupported
// ops fail closed before any bytes are relayed.
func (p *provider) NewStreamRestorer(op string) (wire.StreamRestorer, error) {
	if err := validateMessagesOp(op); err != nil {
		return nil, err
	}
	return &streamRestorer{blocks: make(map[int64]*blockState)}, nil
}

// blockState holds the cross-event holdback for one content block, keyed by the
// stream's content-block index.
//
//   - kind is the content_block.type ("text", "tool_use", "thinking", …),
//     captured at content_block_start so a stop/flush knows how to drain.
//   - textTail is the held partial-token suffix of a text block: the trailing
//     bytes of the concatenated text_delta payloads that could still grow into a
//     CLK_ token, carried forward until proven complete or flushed.
//   - jsonBuf accumulates a tool_use block's input_json_delta fragments so the
//     COMPLETE reconstructed JSON can be restored at block stop (tools need
//     complete input, and full-parse restore is escaping-correct).
type blockState struct {
	kind     string
	textTail []byte
	jsonBuf  bytes.Buffer
}

// streamRestorer is the Anthropic StreamRestorer. It is single-writer: one
// instance serves exactly one response stream, driven sequentially by one relay
// goroutine, so it holds no lock.
type streamRestorer struct {
	blocks map[int64]*blockState
}

// Event consumes one complete Anthropic SSE event payload and returns the
// payload(s) to emit downstream. See the dispatch comments below; the contract
// is in wire.StreamRestorer.
func (s *streamRestorer) Event(eventData []byte, restore wire.RestoreFunc) ([][]byte, error) {
	typeRes := gjson.GetBytes(eventData, "type")
	// A payload whose type can't be parsed as JSON (e.g. a bare "[DONE]" the
	// proxy may hand through, or any non-object) passes through unchanged: there
	// is nothing block-shaped to restore or hold.
	if !typeRes.Exists() {
		return [][]byte{eventData}, nil
	}

	switch typeRes.Str {
	case "content_block_start":
		// Register the block so its kind is known when deltas and the stop
		// arrive. Pass the start through unchanged.
		idx := gjson.GetBytes(eventData, "index").Int()
		s.blocks[idx] = &blockState{kind: gjson.GetBytes(eventData, "content_block.type").Str}
		return [][]byte{eventData}, nil

	case "content_block_delta":
		return s.handleDelta(eventData, restore)

	case "content_block_stop":
		return s.handleStop(eventData, restore)

	case "message_stop":
		// Defensively drain any text tails still held (a well-formed stream
		// flushes them at each content_block_stop, but a stream that omits the
		// stops must not leak a held tail). Emit the synthetic deltas before the
		// stop so the client sees the restored text, then the stop.
		out, err := s.drainAll(restore)
		if err != nil {
			return nil, err
		}
		return append(out, eventData), nil

	default:
		// message_start, message_delta, ping, error, and unknown types pass
		// through immediately and unchanged, preserving keepalive cadence and
		// forward compatibility.
		return [][]byte{eventData}, nil
	}
}

// handleDelta dispatches a content_block_delta on its delta.type.
func (s *streamRestorer) handleDelta(eventData []byte, restore wire.RestoreFunc) ([][]byte, error) {
	idx := gjson.GetBytes(eventData, "index").Int()
	deltaType := gjson.GetBytes(eventData, "delta.type").Str

	switch deltaType {
	case "text_delta":
		blk := s.block(idx, "text")
		// Concatenate the held tail with this delta's text, then split off the
		// safe prefix and hold back the new partial-token suffix. This is the
		// same holdback the raw byte restorer applies, but driven per content
		// block on decoded text rather than on raw SSE bytes — so a token split
		// across text_delta events (mid-CLK_/mid-TYPE/mid-hex) is reassembled
		// before any match is attempted.
		combined := append(blk.textTail, []byte(gjson.GetBytes(eventData, "delta.text").Str)...)
		danger := token.PartialSuffixStart(combined)
		// Growth guard, mirroring internal/stream.Restorer: a dangerous suffix
		// longer than any real token cannot complete, so emit the excess to keep
		// the held tail bounded. No real token spans the forced cut.
		if len(combined)-danger > token.MaxTokenLen {
			danger = len(combined) - token.MaxTokenLen
		}
		safe := combined[:danger]
		// Copy the retained tail into a fresh backing array so the (possibly
		// large) delta backing array can be released and the tail stays bounded.
		blk.textTail = append([]byte(nil), combined[danger:]...)

		restoredSafe, err := restore(string(safe))
		if err != nil {
			return nil, err
		}
		if restoredSafe == "" {
			// The entire delta is being held (its text is wholly inside a partial
			// token). Suppress this event; the bytes resurface on a later delta or
			// at the block stop / flush.
			return [][]byte{}, nil
		}
		// Re-encode via sjson: a restored value containing JSON-special chars is
		// escaped correctly, so delta.text stays valid JSON.
		out, err := sjson.SetBytes(eventData, "delta.text", restoredSafe)
		if err != nil {
			return nil, fmt.Errorf("anthropic: set delta.text: %w", err)
		}
		return [][]byte{out}, nil

	case "input_json_delta":
		blk := s.block(idx, "tool_use")
		// Buffer the fragment; emit nothing. The complete JSON is restored and
		// emitted as one consolidated delta at content_block_stop.
		blk.jsonBuf.WriteString(gjson.GetBytes(eventData, "delta.partial_json").Str)
		return [][]byte{}, nil

	case "thinking_delta", "signature_delta":
		// Thinking/control deltas preserve provider-native semantics. Rewriting
		// them would reinterpret non-user text and can invalidate signatures.
		return [][]byte{eventData}, nil

	default:
		// Unknown delta type: pass through unchanged for forward compatibility.
		return [][]byte{eventData}, nil
	}
}

// handleStop drains the block's held state into a synthetic delta (if any) and
// then emits the original stop, deleting the block. The order is [synthetic?,
// stop] so the restored tail/consolidated input arrives before the block closes.
func (s *streamRestorer) handleStop(eventData []byte, restore wire.RestoreFunc) ([][]byte, error) {
	idx := gjson.GetBytes(eventData, "index").Int()
	blk := s.blocks[idx]
	if blk == nil {
		// Stop for a block we never saw start/deltas for: nothing held.
		return [][]byte{eventData}, nil
	}
	delete(s.blocks, idx)

	synthetic, err := s.drainBlock(idx, blk, restore)
	if err != nil {
		return nil, err
	}
	if synthetic != nil {
		return [][]byte{synthetic, eventData}, nil
	}
	return [][]byte{eventData}, nil
}

// drainBlock turns a block's held state into at most one synthetic
// content_block_delta payload, or nil when there is nothing to flush. It does
// not delete the block (callers own lifecycle).
func (s *streamRestorer) drainBlock(idx int64, blk *blockState, restore wire.RestoreFunc) ([]byte, error) {
	switch blk.kind {
	case "tool_use":
		if blk.jsonBuf.Len() == 0 {
			return nil, nil
		}
		consolidated, err := restoreToolInputJSON(blk.jsonBuf.Bytes(), restore)
		if err != nil {
			return nil, err
		}
		blk.jsonBuf.Reset()
		return syntheticInputJSONDelta(idx, consolidated)

	default:
		// Treat every non-tool block as text-shaped for tail flushing. A held
		// text tail at this point is the trailing partial-token suffix; restore
		// it as a complete value (it terminated when the block stopped).
		if len(blk.textTail) == 0 {
			return nil, nil
		}
		restoredTail, err := restore(string(blk.textTail))
		if err != nil {
			return nil, err
		}
		blk.textTail = nil
		if restoredTail == "" {
			return nil, nil
		}
		return syntheticTextDelta(idx, restoredTail)
	}
}

// drainAll flushes every currently-held block as synthetic deltas in ascending
// index order, leaving the blocks registered (message_stop is not a per-block
// stop, and a later content_block_stop, if any, must still find the block — but
// drainBlock resets the buffers so a subsequent drain emits nothing).
func (s *streamRestorer) drainAll(restore wire.RestoreFunc) ([][]byte, error) {
	if len(s.blocks) == 0 {
		return nil, nil
	}
	idxs := make([]int64, 0, len(s.blocks))
	for idx := range s.blocks {
		idxs = append(idxs, idx)
	}
	sort.Slice(idxs, func(i, j int) bool { return idxs[i] < idxs[j] })

	var out [][]byte
	for _, idx := range idxs {
		synthetic, err := s.drainBlock(idx, s.blocks[idx], restore)
		if err != nil {
			return nil, err
		}
		if synthetic != nil {
			out = append(out, synthetic)
		}
	}
	return out, nil
}

// Flush drains any remaining held blocks at end of stream (covers truncated
// streams that ended without content_block_stop / message_stop). It emits
// synthetic deltas in ascending index order and clears the block table.
func (s *streamRestorer) Flush(restore wire.RestoreFunc) ([][]byte, error) {
	out, err := s.drainAll(restore)
	if err != nil {
		return nil, err
	}
	s.blocks = make(map[int64]*blockState)
	return out, nil
}

// block returns the state for idx, lazily creating it with kind when the stream
// omitted (or we have not yet seen) the content_block_start. An existing block
// keeps its already-recorded kind.
func (s *streamRestorer) block(idx int64, kind string) *blockState {
	blk := s.blocks[idx]
	if blk == nil {
		blk = &blockState{kind: kind}
		s.blocks[idx] = blk
	}
	return blk
}

// restoreToolInputJSON restores the string leaves of a complete tool-input JSON
// value and returns the restored JSON bytes. It reuses the buffered-path leaf
// walker (full-parse + sjson) so escaping is correct by construction: a restored
// value containing quotes/backslashes/control bytes is re-serialized as a valid
// JSON string. raw must be a complete JSON value (object/array/scalar).
func restoreToolInputJSON(raw []byte, restore wire.RestoreFunc) ([]byte, error) {
	// Wrap the value under a known key so we can address its leaves with sjson
	// paths and extract the restored value back out. SetRawBytes inserts raw
	// without re-escaping, preserving the original JSON shape.
	wrapped, err := sjson.SetRawBytes([]byte(`{}`), "v", raw)
	if err != nil {
		return nil, fmt.Errorf("anthropic: wrap tool input: %w", err)
	}
	restored, err := restoreStringLeavesInto(wrapped, gjson.GetBytes(wrapped, "v"), "v", restore)
	if err != nil {
		return nil, err
	}
	return []byte(gjson.GetBytes(restored, "v").Raw), nil
}

// syntheticTextDelta builds a content_block_delta/text_delta payload for index
// carrying text, byte-shaped like a real Anthropic delta. sjson escapes text.
func syntheticTextDelta(index int64, text string) ([]byte, error) {
	out := []byte(`{"type":"content_block_delta"}`)
	var err error
	if out, err = sjson.SetBytes(out, "index", index); err != nil {
		return nil, fmt.Errorf("anthropic: build synthetic text_delta index: %w", err)
	}
	if out, err = sjson.SetBytes(out, "delta.type", "text_delta"); err != nil {
		return nil, fmt.Errorf("anthropic: build synthetic text_delta type: %w", err)
	}
	if out, err = sjson.SetBytes(out, "delta.text", text); err != nil {
		return nil, fmt.Errorf("anthropic: build synthetic text_delta text: %w", err)
	}
	return out, nil
}

// syntheticInputJSONDelta builds a content_block_delta/input_json_delta payload
// for index whose partial_json is the complete restored tool-input JSON. The
// JSON is carried as a string value, so sjson escapes it correctly (the field
// is a JSON-string-encoded JSON document, matching Anthropic's wire shape).
func syntheticInputJSONDelta(index int64, completeJSON []byte) ([]byte, error) {
	out := []byte(`{"type":"content_block_delta"}`)
	var err error
	if out, err = sjson.SetBytes(out, "index", index); err != nil {
		return nil, fmt.Errorf("anthropic: build synthetic input_json_delta index: %w", err)
	}
	if out, err = sjson.SetBytes(out, "delta.type", "input_json_delta"); err != nil {
		return nil, fmt.Errorf("anthropic: build synthetic input_json_delta type: %w", err)
	}
	if out, err = sjson.SetBytes(out, "delta.partial_json", string(completeJSON)); err != nil {
		return nil, fmt.Errorf("anthropic: build synthetic input_json_delta partial_json: %w", err)
	}
	return out, nil
}
