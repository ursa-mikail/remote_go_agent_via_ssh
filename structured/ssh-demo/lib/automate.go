package lib

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

// Automate is a simple demo automation workflow
func Automate(client *ssh.Client) error {
	cmds := []string{
		"echo 'Starting automation...'",
		"hostname",
		"uptime",
	}
	for _, c := range cmds {
		code, out, errOut, err := RunRemoteCommand(client, c)
		fmt.Printf("cmd=%q exit=%d\nstdout=%s\nstderr=%s\n", c, code, out, errOut)
		if err != nil {
			return err
		}
	}
	return nil
}
