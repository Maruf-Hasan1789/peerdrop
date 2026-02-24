# PeerDrop

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-green)](https://golang.org)
[![Wails](https://img.shields.io/badge/Wails-v2-blueviolet)](https://wails.io)
[![Latest Release](https://img.shields.io/github/v/release/Maruf-Hasan1789/peerdrop?include_prereleases)](https://github.com/Maruf-Hasan1789/peerdrop/releases)

**PeerDrop** is a fast, peer-to-peer file transfer tool for local networks.  
No cloud. No servers. Just direct LAN transfers using **mDNS** for peer discovery.

---

## 🚀 Why PeerDrop?

- Transfer files directly over LAN
- Avoid USB drives and cloud uploads
- Share builds, logs, and test artifacts instantly

Designed for **everyone** who needs fast, easy LAN file transfers.

---

## ✨ Features

- ⚡ Fast file transfer over local network (No Http)
- 🌐 Automatic peer discovery using **mDNS**
- 📁 Multiple file selection
- 🖥 Cross-platform support (Windows, Linux)
- 🧩 Minimal and intuitive UI

> **Note:** Encryption is **not yet implemented**. Transfers occur only over the local network. Avoid untrusted networks.

---

## 📥 Releases

Download the latest pre-built releases here:  
👉 [PeerDrop Releases](https://github.com/Maruf-Hasan1789/peerdrop/releases)

---

## 🏗 Tech Stack

- **Backend:** Go
- **Frontend / UI:** Wails (Vanilla JS + CSS)
- **Protocol:** TCP
- **Peer Discovery:** mDNS
- **Serialization:** JSON

---

## 🛠 Installation

### Ubuntu / Linux

```bash
sudo apt install peerdropapp
```
### Windows

1. Download the installer from the [Releases page](https://github.com/Maruf-Hasan1789/peerdrop/releases).
2. Run the installer and follow the prompts.
3. Allow permissions for private and public networks

---

### Build from Source (Optional)

#### Prerequisites

- Go 1.24+
- Node.js 18+
- Wails v2

#### Build Steps

```bash
git clone https://github.com/Maruf-Hasan1789/peerdrop.git
cd peerdrop
wails build
# Binary will be in the 'build/bin' folder
