# `yeet` your file across the interweb

A blazingly fast, zero-setup, peer-to-peer (P2P) file transfer tool powered by modern WebRTC.

> [!WARNING]  
> ⚠️ **Under Construction:** Wear a hard hat and expect flying objects! Absolutely not ready for production yet.

---

## Basic Usage

NAT hole-punching and secure handshakes are handled automatically under the hood.

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
Fling your file into the void by specifying its name. Enter the receiver's code, and watch it fly:

```bash
yeet my_cat_photo.jpg
```

---

## Future Ideas
- **Local send:** Automatically discover and transfer directly over local networks (LAN) without external signalling.
- **Multi-file transfer:** Fling batches of files or whole directories at once.
- **Self-hosted signalling server:** CLI configurations to easily point clients to your own private signalling nodes.
- **Resume transfer:** Seamlessly pick up right where you left off if a connection gets interrupted mid-flight.
