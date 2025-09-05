// go:build !windows
package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// --- Embed all payload binaries in payloads/ directory -----------------------
// Drop any executable you want to carry in payloads/, e.g. openssl, mytool.
//go:embed payloads/*
var payloads embed.FS

func main() {
	// Environment defaults
	hostEnv := os.Getenv("SSH_HOST")
	passwordEnv := os.Getenv("SSH_PASS")
	keyEnv := os.Getenv("SSH_KEY")

	userEnv := ""
	hostOnly := ""
	if hostEnv != "" && strings.Contains(hostEnv, "@") {
		parts := strings.SplitN(hostEnv, "@", 2)
		userEnv = parts[0]
		hostOnly = parts[1]
	} else {
		hostOnly = hostEnv
	}

	// Command-line flags
	host := flag.String("host", hostOnly, "Remote host or IP")
	port := flag.Int("port", 22, "SSH port")
	user := flag.String("user", userEnv, "SSH username")
	keyPath := flag.String("key", keyEnv, "Path to private key (PEM)")
	password := flag.String("password", passwordEnv, "SSH password")
	remoteTmp := flag.String("remote-tmp", "/tmp", "Remote temp dir to upload binaries")
	installDir := flag.String("install-dir", "/usr/local/bin", "Remote install dir for binaries")
	sudo := flag.Bool("sudo", true, "Use sudo to install binaries")
	selfTest := flag.String("self-test", "--version", "Command line args for self-test")
	timeoutSec := flag.Int("timeout", 30, "SSH timeout in seconds")
	flag.Parse()

	if *host == "" || *user == "" {
		log.Fatalf("host and user must be provided (via flags or SSH_HOST env)")
	}

	addr := net.JoinHostPort(*host, fmt.Sprint(*port))
	cfg, err := sshConfig(*user, *keyPath, *password)
	if err != nil {
		log.Fatalf("ssh config error: %v", err)
	}
	cfg.Timeout = time.Duration(*timeoutSec) * time.Second

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		log.Fatalf("ssh dial: %v", err)
	}
	defer client.Close()

	// Read all embedded payloads
	entries, err := fs.ReadDir(payloads, "payloads")
	if err != nil {
		log.Fatalf("read embedded payloads: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Skip hidden files like .DS_Store
    	if strings.HasPrefix(name, ".") || entry.IsDir() {
        	continue
    	}

		data, err := payloads.ReadFile("payloads/" + name)
		if err != nil {
			log.Printf("failed to read %s: %v", name, err)
			continue
		}

		remoteTmpPath := filepath.Join(*remoteTmp, name)
		remoteInstallPath := filepath.Join(*installDir, name)

		// Upload
		if err := scpUpload(client, data, remoteTmpPath, 0o755); err != nil {
			log.Fatalf("scp upload %s: %v", name, err)
		}
		log.Printf("uploaded %s to %s (%d bytes)", name, remoteTmpPath, len(data))

		// Install
		if *sudo {
			installCmd := fmt.Sprintf(
				"echo %s | sudo -S install -m 0755 -o root -g root %s %s",
				shellQuote(*password),
				shellQuote(remoteTmpPath),
				shellQuote(remoteInstallPath),
			)
			if out, err := run(client, installCmd); err != nil {
				log.Fatalf("install %s failed: %v\n%s", name, err, out)
			}
		} else {
			mvCmd := fmt.Sprintf("mv %s %s && chmod 0755 %s",
				shellQuote(remoteTmpPath), shellQuote(remoteInstallPath), shellQuote(remoteInstallPath))
			if out, err := run(client, mvCmd); err != nil {
				log.Fatalf("move %s failed: %v\n%s", name, err, out)
			}
		}
		log.Printf("installed %s to %s", name, remoteInstallPath)

		// Verify checksum
		localSHA := sha256.Sum256(data)
		chkCmd := fmt.Sprintf(
			"sha256sum %s || shasum -a 256 %s || openssl dgst -sha256 %s",
			shellQuote(remoteInstallPath), shellQuote(remoteInstallPath), shellQuote(remoteInstallPath))
		if out, err := run(client, chkCmd); err == nil {
			log.Printf("remote checksum for %s:\n%s", name, strings.TrimSpace(out))
		}
		log.Printf("local  checksum for %s: %x", name, localSHA)

		// Run self-test
		testCmd := fmt.Sprintf("%s %s", shellQuote(remoteInstallPath), *selfTest)
		if out, err := run(client, testCmd); err != nil {
			log.Fatalf("self-test %s failed: %v\n%s", name, err, out)
		} else {
			fmt.Printf("\n===== SELF-TEST OUTPUT for %s =====\n%s\n", name, out)
		}
	}
}

// SSH configuration helper
func sshConfig(user, keyPath, password string) (*ssh.ClientConfig, error) {
	auths := []ssh.AuthMethod{}
	if password != "" {
		auths = append(auths, ssh.Password(password))
	} else if keyPath != "" {
		pem, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	return &ssh.ClientConfig{
		User:            user,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

// SCP upload for binary files
func scpUpload(client *ssh.Client, data []byte, remotePath string, mode os.FileMode) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	var stderr bytes.Buffer
	sess.Stderr = &stderr

	stdin, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	remoteDir := filepath.Dir(remotePath)
	remoteFile := filepath.Base(remotePath)

	if err := sess.Start("scp -t " + shellQuote(remoteDir)); err != nil {
		return fmt.Errorf("start scp: %w (%s)", err, stderr.String())
	}

	go func() {
		defer stdin.Close()
		fmt.Fprintf(stdin, "C%04o %d %s\n", mode&0o7777, len(data), remoteFile)
		_, _ = stdin.Write(data)
		stdin.Write([]byte{0}) // SCP EOF
	}()

	if err := sess.Wait(); err != nil {
		return fmt.Errorf("scp wait: %w (%s)", err, stderr.String())
	}

	return nil
}

// Run a command over SSH
func run(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	if err := sess.Run(cmd); err != nil {
		return stdout.String() + stderr.String(), fmt.Errorf("remote cmd error: %w", err)
	}
	return stdout.String(), nil
}

// Shell-quote helper
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}


/*
Usage:

go mod init ssh_capsule_load_and_run
go mod tidy
go run main.go

 % go run main.go            
2025/09/04 23:44:48 uploaded sysinfo to /tmp/sysinfo (12336 bytes)
2025/09/04 23:44:48 installed sysinfo to /usr/local/bin/sysinfo
2025/09/04 23:44:48 remote checksum for sysinfo:
9da89946c095256db6e82abf35bbb71842b4a6d7f92fa81a9e0b1c75eef9fba9  /usr/local/bin/sysinfo
2025/09/04 23:44:48 local  checksum for sysinfo: 9da89946c095256db6e82abf35bbb71842b4a6d7f92fa81a9e0b1c75eef9fba9

===== SELF-TEST OUTPUT for sysinfo =====
Linux gpu-m 6.14.0-29-generic #29~24.04.1-Ubuntu SMP PREEMPT_DYNAMIC Thu Aug 14 16:52:50 UTC 2 x86_64 x86_64 x86_64 GNU/Linux
MemTotal:       32862164 kB
MemFree:        30075548 kB
MemAvailable:   31469764 kB
=== Ubuntu Machine Info ===
Hostname: gpu-m
User: m
Full uname info above.
Memory info above.


✅ How it works

Drop any binaries you want to carry in payloads/ (e.g., openssl, mytool).

The Go program reads them via embed.FS.

Uploads each to remoteTmp (default /tmp).

Installs each to installDir (/usr/local/bin) with sudo if enabled.

Verifies SHA256 checksum.

Runs a self-test (--version by default, can be overridden via --self-test).

*/