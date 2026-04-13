# qsend

A memory-safe, application-layer file transfer protocol built over QUIC. 

`qsend` bypasses legacy TCP bottlenecks by utilizing QUIC stream multiplexing. It implements custom binary serialization and zero-allocation disk streaming to ensure high-throughput data delivery without causing memory exhaustion on the host machine.

## Architecture

This protocol is built on three strict engineering pillars:

1.  **Transport & Cryptography:** Built on QUIC and TLS 1.3. Enforces strict X.509 certificate validation for the transport tunnel and implements SHA-256 payload hashing for end-to-end data integrity.
2.  **Memory Management:** Utilizes standard `io.Reader` and `io.Writer` interfaces to stream data directly from the network socket to the physical disk (`io.Copy`). File payloads are never buffered into the application's RAM.
3.  **Custom Binary Envelope:** Implements a strict, variable-length binary header parsed via a multi-stage read to prevent stream decapitation and race conditions.

### Protocol Specification

Data is transmitted over the wire in a tightly packed binary format. The server reads the stream sequentially based on this structure:

* **[ 1 Byte ] Action:** `U` (Upload) or `D` (Download)
* **[ 1 Byte ] Name Length:** Unsigned 8-bit integer representing the length of the filename.
* **[ Variable ] Filename:** The string representation of the file.
* **[ 32 Bytes ] Hash:** The SHA-256 checksum of the file payload.
* **[ Variable ] Payload:** The raw bytes of the target file.

## Build Instructions

To compile the executables, clone the repository and run the build commands from the root directory.

```bash
# Compile the Server
go build -o qsend-server ./cmd/server

# Compile the Client
go build -o qsend-client ./cmd/client
```

> Note: Generating a valid `server.pem` and `server.key` in the server execution directory is required for the TLS 1.3 handshake to succeed.*

## Usage

The system operates on a standard Client-Server model.

### 1. Start the Server
Run the server binary. It will bind to `localhost:1447` and await incoming QUIC connections.

```bash
./qsend-server
```

### 2. Client Upload
To upload a file to the server, pass the `U` flag followed by the absolute or relative path to the local file.

```bash
./qsend-client U /path/to/local/video.mp4
```

### 3. Client Download
To download a file from the server, pass the `D` flag followed by the exact name of the file as it exists on the server.

```bash
./qsend-client D video.mp4
```
