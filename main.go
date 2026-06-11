package protosniff

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

func DecodeProto(content []byte) (map[string]any, error) {
	m, err := DecodeProtoHelper(content)
	if err != nil {
		return make(map[string]any, 0), err
	}
	return m, nil
}

func EncodeProto(m map[string]any) (string, error) {
	protoBytes, err := EncodeProtoHelper(m)
	if err != nil {
		return "", err
	}
	grpcBytes := AddGRPCHeader(protoBytes)
	return hex.EncodeToString(grpcBytes), nil
}

func AddGRPCHeader(protoBytes []byte) []byte {
	header := make([]byte, 5)
	header[0] = 0
	binary.BigEndian.PutUint32(header[1:5], uint32(len(protoBytes)))
	return append(header, protoBytes...)
}

func RemoveGRPCHeader(protoBytes []byte) []byte {

	if len(protoBytes) >= 5 {
		msgLen := binary.BigEndian.Uint32(protoBytes[1:5])
		if int(msgLen) == len(protoBytes)-5 {
			protoBytes = protoBytes[5:]
		}
	}

	return protoBytes
}

func GetMapValue(m map[string]any, keys ...string) (any, error) {
	var cur any = m

	for _, key := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("'%s': not a map", key)
		}

		val, exists := obj[key]
		if !exists {
			return nil, fmt.Errorf("key '%s' not found", key)
		}

		cur = val
	}

	return cur, nil
}
