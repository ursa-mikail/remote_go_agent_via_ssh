# Remote Go Agent via SSH

{remote_agent_upload.go, remoteprog.go} provides a simple tool to upload a Go program to a remote server over SSH and execute it, allowing developers to test or run Go code on remote machines without manually logging in.

remote_key_manager.go: commands a remote server and manages multiple key files (key_01.json … key_0N.json). It can list all available key files on the remote, and download a specified key file on demand. This allows selective retrieval of secrets or data files without manually logging into the server.

{remote_agent_upload_and_run_shamir_split.go, remote_shamir.go}: provides a secure, portable way to split and recombine secrets using Shamir’s Secret Sharing, and allows you to run the program remotely over SSH without requiring Go or internet access on the remote server.

## Prerequisites

- Go 1.20+ installed
- SSH access to the remote server
- SFTP enabled on the server

## Setup

1. Set environment variables:

```bash
export SSH_HOST=<id@IP>
export SSH_PASS=<passcode>
# Optional: export SSH_KEY="$HOME/.ssh/id_rsa"
```

2. Prepare your Go program to run remotely (example `remoteprog.go`):

```go
package main

import (
    "fmt"
    "time"
)

func main() {
    fmt.Println("Hello from remote server!")
    fmt.Println("Current time:", time.Now())
}
```

## Usage

1. Run the uploader program:

```bash
go run main.go
```

2. The program will:
    - Connect to the remote server using the provided SSH credentials
    - Upload `remoteprog.go` to `/tmp/remoteprog.go`
    - Execute `go run /tmp/remoteprog.go` on the remote server
    - Display the output locally

## Notes

- Make sure the remote server has Go installed, or compile your program locally and upload the binary instead.
- For private networks (10.x.x.x), ensure your machine can reach the server via VPN or LAN.
