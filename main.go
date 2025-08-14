package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

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

	// SSH client config with password
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

	// Upload Go file
	localFile := "remoteprog.go"
	remoteFile := "/tmp/remoteprog.go"

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		log.Fatalf("Failed to start SFTP: %v", err)
	}
	defer sftpClient.Close()

	srcFile, err := os.Open(localFile)
	if err != nil {
		log.Fatalf("Failed to open local file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := sftpClient.Create(remoteFile)
	if err != nil {
		log.Fatalf("Failed to create remote file: %v", err)
	}
	defer dstFile.Close()

	data, err := ioutil.ReadAll(srcFile)
	if err != nil {
		log.Fatalf("Failed to read local file: %v", err)
	}

	if _, err := dstFile.Write(data); err != nil {
		log.Fatalf("Failed to write remote file: %v", err)
	}

	fmt.Println("File uploaded successfully")

	// Run remote Go program
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf("go run %s", remoteFile)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Fatalf("Failed to run remote program: %v\nOutput: %s", err, output)
	}

	fmt.Printf("Remote program output:\n%s\n", output)
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
ssh <SSH_HOST>
sudo apt update
sudo apt install -y golang-go
go version

go mod tidy
go run main.go
*/
