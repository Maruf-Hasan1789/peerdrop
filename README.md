# PeerDrop

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-green)](https://golang.org)
[![Wails](https://img.shields.io/badge/Wails-v2-blueviolet)](https://wails.io)

**PeerDrop** is a fast, peer-to-peer file transfer tool for local networks, built for developers and QA engineers.  
No cloud. No servers. Just direct LAN transfers using **mDNS** for peer discovery.

---

## 🚀 Why PeerDrop?

Transferring files between machines on the same network should be simple and fast.

PeerDrop allows you to:
- Transfer files directly over LAN
- Avoid USB drives and cloud uploads
- Share builds, logs, and test artifacts instantly

Designed for: **developers, QA engineers, and local office environments**.

---

## ✨ Features

- ⚡ Fast file transfer over local network
- 🌐 Automatic peer discovery using **mDNS**
- 📁 Multiple file selection
- 🖥 Cross-platform support (Windows, Linux, macOS)
- 🧩 Minimal and intuitive UI

> **Note:** Encryption is **not yet implemented**. Transfers occur only over the local network. Avoid untrusted networks.

---

## 🏗 Tech Stack

- **Backend:** Go
- **Frontend / UI:** Wails (Vanilla JS + CSS)
- **Protocol:** TCP
- **Peer Discovery:** mDNS
- **Serialization:** JSON

---

## 🛠 Installation

### From Source

```bash
git clone https://github.com/yourusername/peerdrop.git
cd peerdrop
# Build the app using Wails
wails build
