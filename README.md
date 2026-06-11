# Protosniff

A zero-dependency Go library for encoding and decoding raw gRPC/protobuf payloads **without a `.proto` file**.

Inspect live traffic, reverse-engineer unknown payloads, or build dynamic protobuf tools — all from binary.

---

## Features

- Decode gRPC binary → `map[string]any` (no `.proto` needed)
- Encode `map[string]any` → gRPC binary
- Automatic gRPC 5-byte LPM header strip / add
- Recursive nested message detection
- Repeated field support (`[]any`)
- Auto-resolves wire type 2 as: string → nested message → raw hex
- Zero external dependencies

---

## Installation

```bash
go get github.com/sidhu-patil/protosniff
```

---

## Quick Start

### Decode gRPC binary → map

```go
package main

import (
    "fmt"
    "github.com/sidhu-patil/protosniff"
)

func main() {
    raw := []

    decoded, err := protosniff.DecodeProto(raw)
    if err != nil {
        panic(err)
    }

    fmt.Println(decoded)
}
```

### Encode map → gRPC binary

```go
package main

import (
    "fmt"
    "github.com/sidhu-patil/protosniff"
)

func main() {

    payload := map[string]any{
        "1": map[string]any{
            "1": "first",
            "2": map[string]any{
                "3": "",
            },
        },
    }

    encoded, err := protosniff.EncodeProto(payload)
    if err != nil {
        panic(err)
    }

    fmt.Println(encoded)
}
```

### Round-trip (decode → modify → re-encode)

```go
decoded, _ := protosniff.DecodeProto(bytes)

// Modify a field
outer := decoded["1"].(map[string]any)
outer["1"] = "new" 

reEncoded, _ := protosniff.EncodeProto(decoded)
```

---

## How It Works

### gRPC wire format

Every gRPC message has a **5-byte Length-Prefixed Message (LPM) header** before the protobuf payload:

```
┌──────────┬──────────────────┬─────────────────────┐
│  Byte 0  │    Bytes 1–4     │       Bytes 5+       │
│  Compress│  Message Length  │   Protobuf payload   │
│  flag    │  (big-endian u32)│   (encoded fields)   │
└──────────┴──────────────────┴─────────────────────┘
```

`protosniff` strips this header on decode and re-adds it on encode automatically.

### Protobuf wire types

Each protobuf field is encoded as a tag + value pair. The tag encodes the field number and wire type:

```
tag = (field_number << 3) | wire_type
```

| Wire Type | Meaning          | Go type returned                           |
| --------- | ---------------- | ------------------------------------------ |
| 0         | Varint           | `uint64`                                   |
| 1         | 64-bit           | `uint64`                                   |
| 2         | Length-delimited | `string` / `map[string]any` / hex `string` |
| 5         | 32-bit           | `uint64`                                   |

### Type resolution for wire type 2

When `protosniff` encounters a length-delimited field it resolves the type automatically:

```
raw bytes
    │
    ├── empty?              → ""  (empty string)
    ├── valid printable UTF-8? → string
    ├── valid protobuf?     → map[string]any  (recurse)
    └── else                → hex string
```

---

## Map structure

Field numbers are used as string keys. Nested messages are nested maps. Repeated fields become `[]any`.

```
Protobuf field 1  →  map key "1"
Protobuf field 12 →  map key "12"
Nested message    →  map[string]any
Repeated field    →  []any
Empty string      →  ""
Varint            →  uint64
```

---

## Supported value types for encoding

| Go type          | Encoded as                                           |
| ---------------- | ---------------------------------------------------- |
| `map[string]any` | Nested message (wire type 2)                         |
| `string`         | Length-delimited bytes (wire type 2)                 |
| `uint64`         | Varint (wire type 0)                                 |
| `int`, `int64`   | Varint (wire type 0)                                 |
| `float64`        | Varint (wire type 0, truncated to uint64)            |
| `[]any`          | Repeated field (same field number, multiple entries) |
| `nil`            | Empty length-delimited (wire type 2)                 |

---

## Limitations

Since there is no `.proto` file, the following information is unavailable:

| Lost                 | Reason                                                 |
| -------------------- | ------------------------------------------------------ |
| Field names          | Only field numbers are in the binary                   |
| Enum names           | Encoded as integers                                    |
| `sint32`/`sint64`    | Indistinguishable from regular varint without schema   |
| `fixed32`/`fixed64`  | Indistinguishable from `float`/`double` without schema |
| `oneof` / `map<k,v>` | Look like regular fields in wire format                |

For full fidelity decoding with field names, use the official [protobuf Go library](https://google.golang.org/protobuf) with a `.proto` file.

---

## Use cases

- **Reverse engineering** unknown gRPC APIs
- **Traffic inspection** and manipulation (proxy, middleware)
- **Testing** — forge or mutate gRPC payloads without regenerating stubs
- **Debugging** — human-readable view of live gRPC traffic
- **Dynamic clients** — call gRPC services when you only have the hex payload

---

## Contributing

Pull requests are welcome. For major changes please open an issue first.

```bash
git clone https://github.com/sidhu-patil/protosniff
cd protosniff
go test ./...
```