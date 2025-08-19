package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"strconv"  

	"golang.org/x/crypto/ssh"

	"sshdemo/lib"
)

func main() {
	sshHost := os.Getenv("SSH_HOST")
	sshPass := os.Getenv("SSH_PASS")

	if sshHost == "" || sshPass == "" {
		log.Fatal("SSH_HOST and SSH_PASS must be set")
	}

	user, host := parseUserHost(sshHost)

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "upload":
		uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)
		localPath := uploadCmd.String("local", "", "Local file path")
		remotePath := uploadCmd.String("remote", "", "Remote file path")
		
		uploadCmd.Parse(os.Args[2:])
		
		fmt.Println("DEBUG: LOCAL =", *localPath, "REMOTE =", *remotePath)
		if *localPath == "" || *remotePath == "" {
			fmt.Println("Usage: task upload LOCAL=<file> REMOTE=<path>")
			os.Exit(2)
		}
		// Assuming lib and client are defined elsewhere in your code
		// if err := lib.UploadFile(client, *localPath, *remotePath); err != nil {
		//     log.Fatalf("upload failed: %v", err)
		// }
		fmt.Println("✅ upload complete")


	case "exec":
		fs := flag.NewFlagSet("exec", flag.ExitOnError)
		cmd := fs.String("cmd", "", "command to run on remote")
		_ = fs.Parse(os.Args[2:])
		if *cmd == "" {
			fs.Usage()
			os.Exit(2)
		}
		code, out, errOut, err := lib.RunRemoteCommand(client, *cmd)
		fmt.Printf("exit=%d\n--- stdout ---\n%s\n--- stderr ---\n%s\n", code, out, errOut)
		if err != nil {
			log.Fatalf("remote exec error: %v", err)
		}

	case "shamir":
		// read from environment first (Taskfile.yml passes them)
		secretEnv := os.Getenv("SECRET")
		nEnv := os.Getenv("N")
		kEnv := os.Getenv("K")
		dirEnv := os.Getenv("DIR")

		// fallback to flags for manual CLI
		fs := flag.NewFlagSet("shamir", flag.ExitOnError)
		secret := fs.String("secret", secretEnv, "secret string to split (random hex if empty)")
		n := fs.Int("n", 5, "number of shares")
		k := fs.Int("k", 3, "threshold")
		dir := fs.String("dir", "/tmp/keys", "remote directory to store shares")
		_ = fs.Parse(os.Args[2:])

		// override with env if set
		if secretEnv != "" {
			*secret = secretEnv
		}
		if dirEnv != "" {
			*dir = dirEnv
		}
		if nEnv != "" {
			if val, err := strconv.Atoi(nEnv); err == nil {
				*n = val
			}
		}
		if kEnv != "" {
			if val, err := strconv.Atoi(kEnv); err == nil {
				*k = val
			}
		}

		names, err := lib.CreateShamirShares(client, *secret, *n, *k, *dir)
		if err != nil {
			log.Fatalf("create shares failed: %v", err)
		}
		fmt.Println("✅ created shares:")
		for _, n := range names {
			fmt.Println(" -", n)
		}


	case "listkeys":
		fs := flag.NewFlagSet("listkeys", flag.ExitOnError)
		dir := fs.String("dir", "/tmp/keys", "remote directory containing key_*.json")
		_ = fs.Parse(os.Args[2:])
		names, err := lib.ListRemoteKeys(client, *dir)
		if err != nil {
			log.Fatalf("list keys failed: %v", err)
		}
		if len(names) == 0 {
			fmt.Println("(no key_*.json found)")
			return
		}
		fmt.Println("available key files:")
		for _, n := range names {
			fmt.Println(" -", n)
		}

	case "downloadkey":
			fs := flag.NewFlagSet("downloadkey", flag.ExitOnError)
			dir := fs.String("dir", "/tmp/keys", "remote directory containing key_*.json")
			file := fs.String("file", "", "filename to download (e.g., key_03.json)")
			out := fs.String("out", "", "local output path (default: same name in current dir)")
			_ = fs.Parse(os.Args[2:])
			if *file == "" {
				fs.Usage()
				os.Exit(2)
			}
			dest := *out
			if strings.TrimSpace(dest) == "" {
				dest = *file
			}
			if err := lib.DownloadRemoteKey(client, *dir, *file, dest); err != nil {
				log.Fatalf("download failed: %v", err)
			}
			fmt.Printf("✅ downloaded %s -> %s\n", *file, dest)

	case "monitor":
		s, err := lib.CollectBasicStats(client)
		if err != nil {
			log.Fatalf("monitor failed: %v", err)
		}
		fmt.Println(s)

	case "automate":
		if err := lib.Automate(client); err != nil {
			log.Fatalf("automation failed: %v", err)
		}
		fmt.Println("✅ automation done")

	case "log":
		fs := flag.NewFlagSet("log", flag.ExitOnError)
		msg := fs.String("msg", "", "message to log remotely")
		path := fs.String("file", "/tmp/ssh_demo.log", "remote log file path")
		_ = fs.Parse(os.Args[2:])
		if *msg == "" {
			fs.Usage()
			os.Exit(2)
		}
		if err := lib.WriteRemoteLog(client, *path, *msg); err != nil {
			log.Fatalf("remote log write failed: %v", err)
		}
		fmt.Printf("✅ wrote to %s\n", *path)

	default:
		usage()
	}
}

func usage() {
	fmt.Println(`usage:
  go run main.go <command> [flags]

commands:
  upload       -local <path> -remote <path>
  exec         -cmd "<remote command>"
  shamir       [-secret <s>] [-n 5] [-k 3] [-dir /tmp/keys]
  listkeys     [-dir /tmp/keys]
  downloadkey  -file key_XX.json [-dir /tmp/keys] [-out <local>]
  monitor
  automate
  log          -msg "<text>" [-file /tmp/ssh_demo.log]
`)
}

// parseUserHost splits "user@host" into user and host
func parseUserHost(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return s[:i], s[i+1:]
		}
	}
	log.Fatalf("Invalid SSH_HOST format: %s", s)
	return "", ""
}


/*
go mod init sshdemo
go mod tidy
go run main.go

*/