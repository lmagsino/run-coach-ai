// Command hash-password prints the bcrypt hash of a password read from stdin,
// for RUNCOACH_OWNER_PASSWORD_HASH.
//
//	echo -n 'your password' | go run ./cmd/hash-password
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
)

func main() {
	in, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && in == "" {
		log.Fatalf("read password from stdin: %v", err)
	}
	password := strings.TrimRight(in, "\r\n")
	if password == "" {
		log.Fatal("empty password")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(hash)
}
