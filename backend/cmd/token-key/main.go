// Command token-key prints a fresh RUNCOACH_TOKEN_KEY.
package main

import (
	"fmt"
	"log"

	"github.com/lmagsino/run-coach-ai/backend/internal/crypto"
)

func main() {
	key, err := crypto.NewKey()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(key)
}
