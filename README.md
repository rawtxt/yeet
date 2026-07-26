# `yeet` your file across the interweb

`yeet` is a fast, zero-setup, peer-to-peer (P2P) file transfer tool powered by modern WebRTC.
`yeet` also works without an internet connection if both the receiver and sender are on the same local area network.

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
Run `yeet` without arguments to wait for a payload. It registers your session and gives you a 3-word phrase code:

```bash
yeet
```

*Output:*
```text
Your Session ID: alert-aware-bacon
Local IP: 192.168.1.50
Waiting for a sender to connect...
```

### Send
Yeet files directly to your receiver by specifying filenames:

```bash
yeet cat.jpg book.pdf
```

*Output:*
```text
Enter Session ID: alert-aware-bacon
🔗 Connected to signalling server! Handshaking with receiver...
```

### Direct IP / LAN Transfer
Connect directly to a receiver's IP address on the local network (bypassing the public matchmaker):

```bash
yeet -receiver-ip 192.168.1.50 cat.jpg
```

### Self-Hosted Matchmaker Server
You can run your own combined matchmaking server (signalling HTTP server + STUN UDP server):

```bash
yeet -run-matchmaker -addr :8080 -stun-addr :3478
```

To point your sender or receiver to a custom matchmaker server:

```bash
# Receive using custom matchmaker
yeet -matchmaker http://localhost:8080

# Send using custom matchmaker
yeet -matchmaker http://localhost:8080 file.zip
```

## CLI Flags

| Flag | Description | Default |
| --- | --- | --- |
| `-run-matchmaker` | Start self-hosted matchmaker server (signalling + STUN) | `false` |
| `-matchmaker <url>` | Custom matchmaker server URL | `https://yeet-server.fly.dev` |
| `-receiver-ip <ip_addr>` | Connect directly to receiver IP address (bypasses external matchmaker) | `""` |
| `-addr <addr>` | Address for matchmaker HTTP server to listen on | `:8080` |
| `-stun-addr <addr>` | Address for matchmaker STUN server to listen on (UDP) | `:3478` |
| `-behind-proxy` | Trust proxy headers for rate limiting (`X-Forwarded-For`, `X-Real-IP`) | `false` |

