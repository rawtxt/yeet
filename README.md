# `yeet` your file across the interweb

Fast, zero-setup, peer-to-peer (P2P) file transfer tool powered by WebRTC.

> [!WARNING]  
> ⚠️ **Under Construction:** Wear a hard hat and expect flying objects! Absolutely not ready for production yet.

---

## About

`yeet` is a fast, zero-setup, peer-to-peer (P2P) file transfer tool powered by modern WebRTC. NAT hole-punching and secure handshakes are handled automatically under the hood.

When operating without an internet connection, `yeet` seamlessly falls back to offline mode. The receiver automatically spins up a local signalling node and advertises itself over the local network via mDNS, allowing the sender to discover and connect directly over LAN with zero configuration.

---

## Installation

Make sure you have [Go](https://go.dev/) installed, then run:

```bash
go install github.com/rawtxt/yeet@latest
```

Or build and install locally from source:

```bash
git clone https://github.com/rawtxt/yeet.git
cd yeet
go install
```

---

## Basic Usage

### 1. Receive
Run `yeet` without arguments to wait for a payload. It registers your session and gives you a 6-digit code:

```bash
yeet
```

*Output:*
```text
🚀 Your 6-digit Session ID: 123456
⏳ Waiting for a sender to connect...
```

### 2. Send
Yeet your file directly to your friend by specifying its name and typing in their 6-digit code:

```bash
yeet my_cat_photo.jpg
```

*Output:*
```text
Enter 6-digit Session ID: 123456
🔗 Connected to signalling server! Handshaking with receiver...
```

### 3. Custom Signalling Node
You can run your own signalling node:

```bash
yeet -signalling -addr :8080
```

To point your clients to a custom signalling node:

```bash
yeet -server http://localhost:8080 [file]
```

---

## Future Ideas
- **Multi-file transfer:** Fling batches of files or whole directories at once.
- **Resume transfer:** Seamlessly pick up right where you left off if a connection gets interrupted mid-flight.
