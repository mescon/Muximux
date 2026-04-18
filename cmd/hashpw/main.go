// hashpw is a simple utility to generate bcrypt password hashes for Muximux config
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	var password string

	// Check if password provided as argument
	if len(os.Args) > 1 {
		password = os.Args[1]
	} else {
		// Read from stdin
		fmt.Print("Enter password: ")

		// Try to read without echo if terminal
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			bytePassword, err := term.ReadPassword(fd)
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}
			password = string(bytePassword)
		} else {
			// Read from pipe/stdin
			reader := bufio.NewReader(os.Stdin)
			var err error
			password, err = reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}
			password = strings.TrimSpace(password)
		}
	}

	if password == "" {
		fmt.Fprintln(os.Stderr, "Password cannot be empty")
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}
