package protosniff

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

func decodeVarint(data []byte, pos int) (uint64, int, error) {
	var result uint64
	var shift uint
	for {
		if pos >= len(data) {
			return 0, 0, fmt.Errorf("unexpected end of data at pos %d", pos)
		}
		b := data[pos]
		pos++
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, 0, fmt.Errorf("varint too long")
		}
	}
	return result, pos, nil
}

func DecodeProtoHelper(data []byte) (map[string]any, error) {
	result := map[string]any{}
	pos := 0

	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return nil, fmt.Errorf("reading tag at pos %d: %w", pos, err)
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)
		key := fmt.Sprintf("%d", fieldNum)

		if fieldNum == 0 {
			return nil, fmt.Errorf("invalid field number 0")
		}

		var value any

		switch wireType {
		case 0: // varint
			v, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return nil, fmt.Errorf("field %d varint: %w", fieldNum, err)
			}
			pos = newPos
			value = v

		case 1: // 64-bit
			if pos+8 > len(data) {
				return nil, fmt.Errorf("field %d: need 8 bytes for 64-bit", fieldNum)
			}
			value = binary.LittleEndian.Uint64(data[pos : pos+8])
			pos += 8

		case 2: // length-delimited
			length, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return nil, fmt.Errorf("field %d length: %w", fieldNum, err)
			}
			pos = newPos
			end := pos + int(length)
			if end > len(data) {
				return nil, fmt.Errorf("field %d: length %d exceeds data", fieldNum, length)
			}
			raw := data[pos:end]
			pos = end
			value = resolveBytes(raw)

		case 5: // 32-bit
			if pos+4 > len(data) {
				return nil, fmt.Errorf("field %d: need 4 bytes for 32-bit", fieldNum)
			}
			value = uint64(binary.LittleEndian.Uint32(data[pos : pos+4]))
			pos += 4

		default:
			return nil, fmt.Errorf("field %d: unknown wire type %d", fieldNum, wireType)
		}

		// Handle repeated fields: if key already exists, convert to []any
		if existing, exists := result[key]; exists {
			switch v := existing.(type) {
			case []any:
				result[key] = append(v, value)
			default:
				result[key] = []any{existing, value}
			}
		} else {
			result[key] = value
		}
	}

	return result, nil
}

// resolveBytes decides what a wire-type-2 payload is:
//   - empty bytes  → ""
//   - valid printable UTF-8 → string
//   - valid nested protobuf → map[string]any (recursive)
//   - else → hex string
func resolveBytes(raw []byte) any {
	if len(raw) == 0 {
		return ""
	}
	if isValidUTF8(raw) && isPrintable(raw) {
		return string(raw)
	}
	if nested, err := DecodeProtoHelper(raw); err == nil && len(nested) > 0 {
		return nested
	}
	return hex.EncodeToString(raw)
}

func isValidUTF8(b []byte) bool {
	for i := 0; i < len(b); {
		c := b[i]
		var size int
		switch {
		case c < 0x80:
			size = 1
		case c&0xE0 == 0xC0 && i+1 < len(b) && b[i+1]&0xC0 == 0x80:
			size = 2
		case c&0xF0 == 0xE0 && i+2 < len(b) && b[i+1]&0xC0 == 0x80 && b[i+2]&0xC0 == 0x80:
			size = 3
		case c&0xF8 == 0xF0 && i+3 < len(b) && b[i+1]&0xC0 == 0x80 && b[i+2]&0xC0 == 0x80 && b[i+3]&0xC0 == 0x80:
			size = 4
		default:
			return false
		}
		i += size
	}
	return true
}

func isPrintable(b []byte) bool {
	for _, c := range b {
		if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}
