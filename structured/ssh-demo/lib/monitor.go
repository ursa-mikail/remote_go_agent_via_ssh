package lib

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// CollectBasicStats runs common commands on remote host
func CollectBasicStats(client *ssh.Client) (string, error) {
	parts := []struct {
		title string
		cmd   string
	}{
		{"uname -a", "uname -a"},
		{"uptime", "uptime"},
		{"cpu/mem", "cat /proc/meminfo 2>/dev/null | head -n 5 || vm_stat"},
		{"disk", "df -h | sed -n '1p;/^[^F]/p'"},
	}

	var b strings.Builder
	b.WriteString("=== remote system report ===\n")
	for _, p := range parts {
		code, out, errOut, err := RunRemoteCommand(client, p.cmd)
		fmt.Fprintf(&b, "\n# %s (exit %d)\n", p.title, code)
		if err != nil && errOut != "" {
			fmt.Fprintf(&b, "%s\n", strings.TrimSpace(errOut))
		}
		fmt.Fprintf(&b, "%s\n", strings.TrimSpace(out))
	}
	return b.String(), nil
}
