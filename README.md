<div align="center">
<h1>qsend</h1>

A file transfer tool built over QUIC. Streams data directly from disk to the network socket with zero in-memory buffering, making it suitable for large file transfers without causing memory pressure on the host.

[![Go Report Card](https://goreportcard.com/badge/github.com/so1icitx/qsend)](https://goreportcard.com/report/github.com/so1icitx/qsend)
[![License](https://img.shields.io/github/license/so1icitx/qsend)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/so1icitx/qsend)](go.mod)

</div>

## How it works

The client opens a QUIC stream to the server and sends a fixed binary header, followed by the raw file payload. The server reads the header, determines whether the client wants to upload or download, and acts accordingly. All file I/O goes through `io.Reader`/`io.Writer` interfaces piped directly into `io.Copy` ,  no intermediate buffers.

**Upload:** The client hashes the file with SHA-256 before sending, embeds the checksum in the header, and streams the file. The server writes the incoming bytes to disk, re-hashes the result, and compares against the header checksum. A mismatch returns a `Hash mismatch` response.

**Download:** The server opens the requested file and streams it to the client. Integrity verification on downloads is not implemented at the application layer ,  QUIC's transport frames handle packet-level integrity only.

A live throughput display prints MB/s to stdout every second during transfer.

## Binary protocol

```
[ 1 byte  ]  Action      ,  'U' (upload) or 'D' (download)
[ 1 byte  ]  NameLen     ,  length of the filename in bytes
[ N bytes ]  Filename    ,  UTF-8 filename string
[ 32 bytes]  Hash        ,  SHA-256 of the file payload (uploads only; zeroed for downloads)
[ ...     ]  Payload     ,  raw file bytes, streamed until EOF
```

## Build

```bash
git clone https://github.com/so1icitx/qsend.git
cd qsend
go build -o qsend-server ./cmd/server
go build -o qsend-client ./cmd/client
```

## Usage

Start the server:

```bash
./qsend-server
```

The server generates a self-signed TLS certificate in memory at startup. No certificate files are required. The client connects with certificate verification disabled (`InsecureSkipVerify`). For production use, export the certificate from the server and configure the client to verify it ,  the relevant code is present in both files, commented out.

Upload a file:

```bash
./qsend-client U ./video.mp4
```

Download a file:

```bash
./qsend-client D video.mp4
```

> The server listens on `localhost:1447`.

## Known limitations 

- Download path has no application-layer integrity check
- No retransmission or retry logic if the server reports a hash mismatch
- Single stream per connection; no concurrent transfers
- Filename is capped at 255 bytes (uint8 length field)
