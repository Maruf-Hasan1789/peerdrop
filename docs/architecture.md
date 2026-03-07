# PeerDrop — Application Architecture

## Overview

PeerDrop is a peer-to-peer file transfer application built with **Wails** (Go backend + Vanilla JS frontend). Users discover each other via **mDNS**, select multiple peers, and send multiple files concurrently — each transfer isolated in its own `PeerSession`.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | Wails — Vanilla JS / HTML |
| Backend | Go |
| Discovery | mDNS via ZeroConf (Go) |
| Transport | TCP (raw sockets) |
| Metadata | Protobuf (header encoding) |
| File Data | Raw bytes (chunk payload) |

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Wails Application                        │
│                                                                 │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │                  Frontend (Vanilla JS)                  │   │
│   │                                                         │   │
│   │   ┌───────────────┐        ┌────────────────────────┐   │   │
│   │   │   Peer List   │        │    File Selector       │   │   │
│   │   │               │        │                        │   │   │
│   │   │  ● Peer A  ✓  │        │  file1.zip             │   │   │
│   │   │  ● Peer B  ✓  │        │  image.png             │   │   │
│   │   │  ● Peer C     │        │  document.pdf          │   │   │
│   │   └───────────────┘        └────────────────────────┘   │   │
│   │                                                         │   │
│   │              [ Send to Selected Peers ]                 │   │
│   └──────────────────────┬──────────────────────────────────┘   │
│                          │  Wails Bridge (JS ↔ Go)              │
│   ┌──────────────────────▼──────────────────────────────────┐   │
│   │                   Go Backend                            │   │
│   │                                                         │   │
│   │   ┌──────────────────────────────────────────────────┐  │   │
│   │   │               mDNS Service (ZeroConf)            │  │   │
│   │   │                                                  │  │   │
│   │   │   Advertise self  ◄──────────────►  Discover     │  │   │
│   │   │   on local network                 other peers   │  │   │
│   │   └──────────────────────┬───────────────────────────┘  │   │
│   │                          │  populates                    │   │
│   │                          ▼                               │   │
│   │                    [ Peer Registry ]                     │   │
│   │               { Peer A, Peer B, Peer C, ... }            │   │
│   │                                                         │   │
│   └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Peer Discovery (mDNS / ZeroConf)

```
  Device A                 Local Network                 Device B
  ────────                 ─────────────                 ────────

  ZeroConf                                               ZeroConf
  Advertise ──── mDNS broadcast ──────────────────────►  Browse
                                                         Discover A
  Browse   ◄─────────────────────── mDNS broadcast ────  Advertise

  Peer B added                                           Peer A added
  to Peer Registry                                       to Peer Registry
```

- Each device **advertises** its own service and **browses** for others simultaneously.
- Discovered peers are added to the local **Peer Registry** and surfaced in the frontend Peer List.
- No central server — fully local network, zero configuration.

---

## Concurrent Transfer Architecture

When a user selects **multiple peers** and hits Send, the Go backend spawns one **PeerSession per peer** — all running concurrently.

```
  User selects: Peer A, Peer B
  Files:        file1.zip, image.png
  
  Go Backend
  ──────────
  
  ┌─────────────────────────────────────────────────────────┐
  │                    Session Manager                      │
  │                                                         │
  │     Send(files, [PeerA, PeerB])                         │
  │            │                                            │
  │     ┌──────┴───────┐                                    │
  │     │              │   (goroutines — concurrent)        │
  │     ▼              ▼                                    │
  │  PeerSession    PeerSession                             │
  │  [ Peer A ]     [ Peer B ]                              │
  │     │              │                                    │
  │  TCP Conn       TCP Conn                                │
  │     │              │                                    │
  │  Transfer       Transfer                                │
  │  file1.zip      file1.zip                               │
  │  image.png      image.png                               │
  └─────────────────────────────────────────────────────────┘
```

- Each `PeerSession` manages its own **TCP connection**, **handshake**, and **file transfer state** independently.
- Sessions do not share state — a failure in one session does not affect others.

---

## PeerSession Lifecycle

```
  PeerSession (one per target peer)
  ──────────────────────────────────────────────────────────

  [1] Dial TCP  ──────────────────────────────►  Peer's TCP listener

  [2] Hello exchange
      Send Hello { id, name, port, version }
                         ◄─────────────────── Hello { id, name, port, version }

  [3] Handshake
      Send Handshake { roots, total_size, chunk_size }
                         ◄─────────────────── Control { HANDSHAKE_ACK, mode, files? }

  [4] File Transfer  ★ raw bytes here
      ┌─────────────────────────────────────────────────────┐
      │  for each permitted file:                           │
      │      for each chunk:                                │
      │          Send [ totalLen ][ hdrLen ][ Chunk ][ raw ]│
      │          Receive Ack (optional)                     │
      └─────────────────────────────────────────────────────┘

  [5] Done
      Send MESSAGE_TYPE_DONE
      Close TCP connection
```

---

## Multi-File + Multi-Peer Concurrency

```
                        Session Manager
                              │
          ┌───────────────────┼───────────────────┐
          │                   │                   │
          ▼                   ▼                   ▼
   PeerSession(A)      PeerSession(B)      PeerSession(C)
          │                   │                   │
     TCP Conn A          TCP Conn B          TCP Conn C
          │                   │                   │
    ┌─────┴──────┐      ┌─────┴──────┐      ┌─────┴──────┐
    │  file1.zip │      │  file1.zip │      │  file1.zip │
    │  image.png │      │  image.png │      │  image.png │
    │  doc.pdf   │      │  doc.pdf   │      │  doc.pdf   │
    └────────────┘      └────────────┘      └────────────┘

  Each session streams all files independently over its own connection.
  All sessions run as concurrent goroutines.
```

---

## Wire Format (per message)

```
Control messages  (Hello, Handshake, Control, Ack, Done)
──────────────────────────────────────────────────────────
[ totalLen (4B) ][ headerLen (4B) ][ Protobuf bytes ][ ∅ empty ]

File chunk messages
──────────────────────────────────────────────────────────
[ totalLen (4B) ][ headerLen (4B) ][ Chunk protobuf ][ raw file bytes ★ ]
```

---

## Component Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│  Wails App                                                       │
│                                                                  │
│  ┌────────────────────────┐                                      │
│  │  Frontend              │                                      │
│  │  Vanilla JS / HTML     │                                      │
│  │                        │                                      │
│  │  • Peer List UI        │                                      │
│  │  • File Picker         │                                      │
│  │  • Transfer Progress   │                                      │
│  └──────────┬─────────────┘                                      │
│             │ Wails Bridge                                       │
│  ┌──────────▼─────────────────────────────────────────────────┐  │
│  │  Go Backend                                                │  │
│  │                                                            │  │
│  │  ┌─────────────────┐     ┌──────────────────────────────┐  │  │
│  │  │  mDNS Service   │     │       Session Manager        │  │  │
│  │  │  (ZeroConf)     │     │                              │  │  │
│  │  │                 │     │  spawn PeerSession per peer  │  │  │
│  │  │  • Advertise    │     │  goroutines — concurrent     │  │  │
│  │  │  • Browse       │     └──────────────┬───────────────┘  │  │
│  │  │  • Peer Registry│                    │                  │  │
│  │  └─────────────────┘          ┌─────────┴──────────┐       │  │
│  │                               │                    │       │  │
│  │                        PeerSession(A)       PeerSession(B) │  │
│  │                               │                    │       │  │
│  │                          TCP Conn A           TCP Conn B   │  │
│  │                               │                    │       │  │
│  │                     ┌─────────┴──────┐   ┌─────────┴────┐  │  │
│  │                     │ Hello          │   │ Hello        │  │  │
│  │                     │ Handshake      │   │ Handshake    │  │  │
│  │                     │ Chunk transfer │   │ Chunk xfer   │  │  │
│  │                     │ (Protobuf +    │   │ (Protobuf +  │  │  │
│  │                     │  raw bytes)    │   │  raw bytes)  │  │  │
│  │                     └────────────────┘   └──────────────┘  │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘

                         Local Network
              ┌─────────────────────────────────┐
              │  Peer A  ←── mDNS ──►  Peer B   │
              │             (ZeroConf)           │
              └─────────────────────────────────┘
```