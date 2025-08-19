package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/hashicorp/vault/shamir"
)

func main() {
	// Example secret: random 32-byte hex string
	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		log.Fatalf("Failed to generate random secret: %v", err)
	}
	secretHex := hex.EncodeToString(secretBytes)
	fmt.Println("Original secret:", secretHex)

	// Split secret into 5 shares, requiring any 3 to reconstruct
	shares, err := shamir.Split([]byte(secretHex), 5, 3)
	if err != nil {
		log.Fatalf("Failed to split secret: %v", err)
	}

	fmt.Println("\nShares:")
	for i, share := range shares {
		fmt.Printf("Share %d: %x\n", i+1, share)
	}

	// Simulate recombining using first 3 shares
	recovered, err := shamir.Combine(shares[:3])
	if err != nil {
		log.Fatalf("Failed to combine shares: %v", err)
	}

	fmt.Println("\nRecovered secret:", string(recovered))
	if string(recovered) == secretHex {
		fmt.Println("✅ Secret successfully recovered!")
	} else {
		fmt.Println("❌ Recovery failed!")
	}
}
