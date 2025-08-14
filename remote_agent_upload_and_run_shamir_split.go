package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func main() {
	sshHost := os.Getenv("SSH_HOST")
	sshPass := os.Getenv("SSH_PASS")

	if sshHost == "" || sshPass == "" {
		log.Fatal("SSH_HOST and SSH_PASS must be set")
	}

	user, host := parseUserHost(sshHost)

	// Build the Go executable locally from remote_shamir.go
	localExe := "remote_shamir_tool"
	buildCmd := exec.Command("go", "build", "-o", localExe, "remote_shamir.go")
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64") // for linux env

	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	fmt.Println("Building local executable from remote_shamir.go...")
	if err := buildCmd.Run(); err != nil {
		log.Fatalf("Failed to build executable: %v", err)
	}

	fmt.Println("Executable built:", localExe)

	// SSH client
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

	// Start SFTP
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		log.Fatalf("Failed to start SFTP: %v", err)
	}
	defer sftpClient.Close()

	remoteFile := "/tmp/" + localExe
	uploadFile(sftpClient, localExe, remoteFile)

	// Make remote executable
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	fmt.Println("Making remote file executable...")
	session.Run(fmt.Sprintf("chmod +x %s", remoteFile))

	// Run remote executable
	session2, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session2.Close()

	fmt.Println("Running remote executable...")
	output, err := session2.CombinedOutput(remoteFile)
	if err != nil {
		log.Fatalf("Failed to run remote executable: %v\nOutput: %s", err, output)
	}

	fmt.Printf("Remote output:\n%s\n", output)
}

func uploadFile(sftpClient *sftp.Client, localPath, remotePath string) {
	srcFile, err := os.Open(localPath)
	if err != nil {
		log.Fatalf("Failed to open local file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := sftpClient.Create(remotePath)
	if err != nil {
		log.Fatalf("Failed to create remote file: %v", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		log.Fatalf("Failed to write remote file: %v", err)
	}

	fmt.Println("Uploaded executable to remote:", remotePath)
}

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
ssh <SSH_HOST>
sudo apt update
sudo apt install -y golang-go
go version

go mod tidy
go run main.go

% go run remote_agent_upload_and_run_shamir_split.go
Building local executable from remote_shamir.go...
Executable built: remote_shamir_tool
Uploaded executable to remote: /tmp/remote_shamir_tool
Making remote file executable...
Running remote executable...
Remote output:
Original secret: e52847a15402d7d0104aac49756f90dd

Shares:
Share 1: 28cbc8f25a5572053b2c2e4a842f0113254e453b30bff0e0df4d47bf23a6fc5c5c
Share 2: 3c427923bd864b956f2a2d5dc825f2c1dc9efe1b9f9e136ccef94a71de940d7688
Share 3: 3eba9fee523c1a9d4258a326e62598c1576ffd2deec7776343cf14bf16cbcff976
Share 4: 257ffd38bef3498b6e24bc496076f42c40d385a97d927469a548ce81f2a63611d0
Share 5: 0cea44ab9d1fa9b10c82980d6c0f149e6414558e2766c5049680c787472dbe0f54

Recovered secret: e52847a15402d7d0104aac49756f90dd
✅ Secret successfully recovered!

*/
