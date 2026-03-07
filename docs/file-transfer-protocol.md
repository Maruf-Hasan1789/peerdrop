# PeerDrop Transfer Protocol (PTP)

PeerDrop uses a custom hybrid protocol — the **PeerDrop Transfer Protocol (PTP)** — combining Protobuf-encoded headers for metadata and raw bytes for file payload. The protocol is designed for efficient, low-overhead, peer-to-peer file transfers over TCP.

---

## Table of Contents

1. [Design Goals](#1-design-goals)
2. [TCP Framing](#2-tcp-framing)
3. [Message Types](#3-message-types)
4. [Protocol Phases](#4-protocol-phases)
    - [4.1 Hello](#41-hello)
    - [4.2 Handshake](#42-handshake)
    - [4.3 File Transfer](#43-file-transfer)
    - [4.4 Completion](#44-completion)
5. [Control Messages](#5-control-messages)
6. [Resume & Recovery](#6-resume--recovery)
7. [Full Exchange Example](#7-full-exchange-example)

---

## 1. Design Goals

| Goal | Description |
|---|---|
| **Low overhead** | File payload is sent as raw bytes — no base64 or envelope encoding for bulk data |
| **Clear boundaries** | Every message is length-prefixed to prevent TCP stream ambiguity |
| **Permission control** | Receiver explicitly grants or restricts access per file before any data is sent |
| **Extensibility** | Protobuf headers allow new fields to be added without breaking existing clients |
| **Resumability** | Protocol includes a resume mechanism for interrupted transfers *(not yet implemented)* |

---

## 2. TCP Framing

Every message — regardless of type — is wrapped in the same frame structure:

```
┌─────────────────┬─────────────────┬────────────────────────┬──────────────────────┐
│  totalLen       │  headerLen      │  Protobuf Header       │  Raw Payload         │
│  (4 bytes)      │  (4 bytes)      │  (variable)            │  (variable)          │
└─────────────────┴─────────────────┴────────────────────────┴──────────────────────┘
```

| Field | Size | Description |
|---|---|---|
| `totalLen` | 4 bytes | Total byte length of the entire message (header + payload) |
| `headerLen` | 4 bytes | Byte length of the Protobuf-encoded header |
| Protobuf Header | variable | Encoded metadata specific to the message type |
| Raw Payload | variable | File chunk bytes — **present only during file transfer** |

> **Note:** Control messages (Hello, Handshake, Control, Ack, Done) always carry an **empty payload**. Raw bytes appear exclusively in file chunk messages.

**Reading algorithm:**
```
1. Read 4 bytes → totalLen
2. Read 4 bytes → headerLen
3. Read headerLen bytes → decode as Protobuf header
4. Read (totalLen - headerLen) bytes → raw payload (if any)
```

---

## 3. Message Types

| Constant | Direction | Description |
|---|---|---|
| `MESSAGE_TYPE_HELLO` | Both | Identity exchange at connection start |
| `MESSAGE_TYPE_HANDSHAKE` | Sender → Receiver | Propose a file transfer with metadata |
| `MESSAGE_TYPE_CONTROL` | Both | Permission responses and transfer control |
| `MESSAGE_TYPE_CHUNK` | Sender → Receiver | A single file chunk (header + raw bytes) |
| `MESSAGE_TYPE_ACK` | Receiver → Sender | Optional per-chunk acknowledgement |
| `MESSAGE_TYPE_DONE` | Sender → Receiver | Signals end of transfer |
| `MESSAGE_TYPE_RESUME` | Receiver → Sender | Request retransmission of missing chunks *(not yet implemented)* |

---

## 4. Protocol Phases

### 4.1 Hello

The Hello phase establishes mutual identity between peers. Neither message carries a payload.

**Flow:**
```
Sender                                        Receiver
──────                                        ────────
Hello { id, name, port, version, ... } ──────►
                               ◄────── Hello { id, name, port, version, ... }
```

**Frame:**
```
[ totalLen ][ headerLen ][ Hello protobuf bytes ][ ∅ ]
```

**`Hello` message fields:**

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique device identifier |
| `name` | string | Human-readable device name |
| `port` | uint32 | TCP port this peer is listening on |
| `version` | string | PeerDrop protocol version |
| `user_name` | string | Display name of the user |

---

### 4.2 Handshake

The Handshake phase lets the sender describe the intended transfer and the receiver grant or restrict permissions before any file data is sent.

**Flow:**
```
Sender                                            Receiver
──────                                            ────────
Handshake { roots, total_size, chunk_size } ──────►
                     ◄────── Control { HANDSHAKE_ACK, mode, files? }
```

**Frame:**
```
[ totalLen ][ headerLen ][ Handshake protobuf bytes ][ ∅ ]
[ totalLen ][ headerLen ][ Control protobuf bytes   ][ ∅ ]
```

**`Handshake` message fields:**

| Field | Type | Description |
|---|---|---|
| `roots` | repeated Entry | List of root files and directories to be transferred |
| `total_size` | uint64 | Total byte size of all files combined |
| `chunk_size` | uint32 | Chunk size in bytes that will be used during transfer |

**`Control` (permission response) fields:**

| Field | Type | Description |
|---|---|---|
| `action` | ControlAction | `CONTROL_ACTION_HANDSHAKE_ACK` |
| `mode` | PermissionMode | `PERMISSION_MODE_NONE` · `PARTIAL` · `ALL` |
| `files` | repeated FileControl | Per-file permission overrides (used when `mode = PARTIAL`) |

Only files explicitly permitted by the receiver will be transferred.

---

### 4.3 File Transfer

Each permitted file is split into chunks and streamed sequentially. This is the **only phase that carries raw bytes** in the payload.

**Flow:**
```
Sender                                        Receiver
──────                                        ────────
[ Chunk header ][ raw bytes ] ───────────────►  verify checksum → write to disk
                               ◄────────────── Ack (optional)
[ Chunk header ][ raw bytes ] ───────────────►  verify checksum → write to disk
                               ◄────────────── Ack (optional)
              ... repeat for all chunks of all permitted files ...
```

**Frame:**
```
[ totalLen ][ headerLen ][ Chunk protobuf bytes ][ raw file bytes ★ ]
```

**`Chunk` header fields:**

| Field | Type | Description |
|---|---|---|
| `root_id` | string | Identifies the root entry this chunk belongs to |
| `file_id` | string | Identifies the specific file within the root |
| `index` | uint32 | Zero-based chunk sequence number |
| `total` | uint32 | Total number of chunks for this file |
| `offset` | uint64 | Byte offset of this chunk within the file |
| `size` | uint32 | Byte length of the raw payload that follows |
| `checksum` | bytes | Hash of the raw payload for integrity verification |

**Concrete frame example** (1 MB chunk, 100-byte header):

```
[ totalLen = 1,048,680 ][ headerLen = 100 ][ 100 bytes Chunk header ][ 1,048,576 bytes raw data ]
```

The receiver reads `totalLen`, then `headerLen`, decodes the Chunk header, reads exactly `size` bytes of raw payload, verifies the checksum, and writes to disk.

---

### 4.4 Completion

Once all chunks have been sent, the sender optionally signals the end of the transfer.

**Frame:**
```
[ totalLen ][ headerLen ][ Done protobuf bytes ][ ∅ ]
```

The TCP connection is closed after this message.

---

## 5. Control Messages

`MESSAGE_TYPE_CONTROL` messages can be sent at any point during an active session to manage transfer state.

| `ControlAction` | Direction | Description |
|---|---|---|
| `CONTROL_ACTION_HANDSHAKE_ACK` | Receiver → Sender | Confirms handshake; carries permission response |
| `CONTROL_ACTION_PAUSE` | Either | Pause the active transfer *(not yet implemented)* |
| `CONTROL_ACTION_RESUME` | Either | Resume a paused transfer *(not yet implemented)* |
| `CONTROL_ACTION_ERROR` | Either | Signal a transfer error *(not yet implemented)* |

All control messages use the standard TCP frame with an empty payload.

---

## 6. Resume & Recovery

> **Status: Not yet implemented.** The framing and message type are defined; the logic is planned for a future release.

If a transfer is interrupted, the receiver can request retransmission of specific chunks:

```
Receiver ──── MESSAGE_TYPE_RESUME { missing_indexes: [4, 7, 12] } ────► Sender
Sender   ──── retransmits only the requested chunks ─────────────────► Receiver
```

**Frame:**
```
[ totalLen ][ headerLen ][ Resume protobuf bytes ][ ∅ ]
```

This allows large transfers to recover from network interruptions without restarting from the beginning.

---

## 7. Full Exchange Example

```
Sender                                          Receiver
──────                                          ────────

── Hello Phase ──────────────────────────────────────────────────────

Hello { id, name, port, version } ────────────────────────────────►
                                  ◄──────────── Hello { id, name, port, version }

── Handshake Phase ──────────────────────────────────────────────────

Handshake { roots, total_size, chunk_size } ───────────────────────►
                                  ◄──────────── Control { HANDSHAKE_ACK, mode=ALL }

── File Transfer Phase ★ raw bytes ──────────────────────────────────

[ Chunk 0/N · file_1 ][ raw bytes ] ──────────────────────────────►  write
                                  ◄──────────── Ack (optional)
[ Chunk 1/N · file_1 ][ raw bytes ] ──────────────────────────────►  write
                                  ◄──────────── Ack (optional)
  ...
[ Chunk N/N · file_1 ][ raw bytes ] ──────────────────────────────►  write
[ Chunk 0/M · file_2 ][ raw bytes ] ──────────────────────────────►  write
  ...
[ Chunk M/M · file_2 ][ raw bytes ] ──────────────────────────────►  write

── Completion ────────────────────────────────────────────────────────

Done ──────────────────────────────────────────────────────────────►
                                                             [close]
```

**Payload presence summary:**

| Phase | Message | Raw Payload |
|---|---|---|
| Hello | `MESSAGE_TYPE_HELLO` | ✗ empty |
| Hello | `MESSAGE_TYPE_HELLO` (reply) | ✗ empty |
| Handshake | `MESSAGE_TYPE_HANDSHAKE` | ✗ empty |
| Handshake | `MESSAGE_TYPE_CONTROL` | ✗ empty |
| Transfer | `MESSAGE_TYPE_CHUNK` | ✓ **raw file bytes** |
| Transfer | `MESSAGE_TYPE_ACK` | ✗ empty |
| Completion | `MESSAGE_TYPE_DONE` | ✗ empty |