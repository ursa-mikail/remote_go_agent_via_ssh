// remoteprog.go
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("Hello from remote server!")
	fmt.Println("Running on:", os.Getenv("HOSTNAME"))
	fmt.Println("Current time:", time.Now().Format(time.RFC1123))
}
