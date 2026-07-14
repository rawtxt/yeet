# `yeet` your file across the interweb

`yeet` is a fast, zero-setup, peer-to-peer (P2P) file transfer tool powered by modern WebRTC.
`yeet` also works without an internet connection. If you are on the same Wi-Fi, `yeet` automatically switches to local offline mode to find your peer and transfer files with zero setup.

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

## Basic Usage

### Receive
Run `yeet` without arguments to wait for a payload. It registers your session and gives you a 6-digit code:

```bash
yeet
```

*Output:*
```text
🚀 Your 6-digit Session ID: 123456
⏳ Waiting for a sender to connect...
```

### Send
Yeet your file directly to your friend by specifying its name and typing in their 6-digit code:

```bash
yeet cat.jpg book.pdf
```

*Output:*
```text
Enter 6-digit Session ID: 123456
🔗 Connected to signalling server! Handshaking with receiver...
```

### Custom Signalling Node
You can run your own signalling node:

```bash
yeet -signalling -addr :8080
```

To point your clients to a custom signalling node:

```bash
yeet -server http://localhost:8080 [file]
```
