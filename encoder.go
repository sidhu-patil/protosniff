package protosniff

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
)

// ─── Varint encoding ───────────────────────────────────────────────
func encodeVarint(v uint64) []byte {
	var buf [10]byte
	n := 0
	for v >= 0x80 {
		buf[n] = byte(v&0x7F) | 0x80
		v >>= 7
		n++
	}
	buf[n] = byte(v)
	return buf[:n+1]
}

// encodeTag encodes (fieldNumber << 3 | wireType) as a varint.
func encodeTag(fieldNum int, wireType int) []byte {
	return encodeVarint(uint64(fieldNum<<3 | wireType))
}

// ─── Core encoder ─────────────────────────────────────────────────

// EncodeProto encodes map[string]any → raw protobuf bytes.
//
// Supported value types:
//
//	map[string]any  → nested message  (wire type 2)
//	string          → length-delimited (wire type 2)
//	uint64/int/float → varint          (wire type 0)
//	[]any           → repeated field  (each element encoded separately)
func EncodeProtoHelper(m map[string]any) ([]byte, error) {
	// Sort keys numerically so field order is deterministic
	keys := make([]int, 0, len(m))
	for k := range m {
		n, err := strconv.Atoi(k)
		if err != nil {
			return nil, fmt.Errorf("non-numeric field key %q", k)
		}
		keys = append(keys, n)
	}
	sort.Ints(keys)

	var out []byte
	for _, fieldNum := range keys {
		key := strconv.Itoa(fieldNum)
		val := m[key]
		encoded, err := encodeField(fieldNum, val)
		if err != nil {
			return nil, fmt.Errorf("field %d: %w", fieldNum, err)
		}
		out = append(out, encoded...)
	}
	return out, nil
}

func encodeField(fieldNum int, val any) ([]byte, error) {
	switch v := val.(type) {

	case map[string]any:
		// Nested message: encode recursively, then wrap as length-delimited
		nested, err := EncodeProtoHelper(v)
		if err != nil {
			return nil, err
		}
		return lengthDelimited(fieldNum, nested), nil

	case string:
		if v == "" {
			// Empty string: tag + length 0
			return lengthDelimited(fieldNum, []byte{}), nil
		}
		// Try hex decode first (raw bytes stored as hex)
		if b, err := hex.DecodeString(v); err == nil && !isPrintableASCII(v) {
			return lengthDelimited(fieldNum, b), nil
		}
		return lengthDelimited(fieldNum, []byte(v)), nil

	case uint64:
		return append(encodeTag(fieldNum, 0), encodeVarint(v)...), nil

	case int:
		return append(encodeTag(fieldNum, 0), encodeVarint(uint64(v))...), nil

	case int64:
		return append(encodeTag(fieldNum, 0), encodeVarint(uint64(v))...), nil

	case float64:
		// JSON numbers unmarshal as float64; treat as uint64 if whole number
		return append(encodeTag(fieldNum, 0), encodeVarint(uint64(v))...), nil

	case []any:
		// Repeated field: encode each element under the same field number
		var out []byte
		for i, elem := range v {
			b, err := encodeField(fieldNum, elem)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out = append(out, b...)
		}
		return out, nil

	case nil:
		return lengthDelimited(fieldNum, []byte{}), nil

	default:
		return nil, fmt.Errorf("unsupported type %T", val)
	}
}

// lengthDelimited wraps payload as: tag(wire=2) + varint(len) + payload
func lengthDelimited(fieldNum int, payload []byte) []byte {
	tag := encodeTag(fieldNum, 2)
	length := encodeVarint(uint64(len(payload)))
	out := make([]byte, 0, len(tag)+len(length)+len(payload))
	out = append(out, tag...)
	out = append(out, length...)
	out = append(out, payload...)
	return out
}

// ─── gRPC LPM header ──────────────────────────────────────────────

// AddGRPCHeader prepends the 5-byte gRPC Length-Prefixed Message header.

// ─── Main entry point ─────────────────────────────────────────────

// EncodeToGRPCHex encodes map[string]any → gRPC hex string (with 5-byte header).

// ─── Helpers ──────────────────────────────────────────────────────

func isPrintableASCII(s string) bool {
	for _, c := range s {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}
