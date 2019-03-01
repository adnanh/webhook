// Hook Stream is a simple utility for testing Webhook streaming capability. It spawns a TCP server on execution
// which echos all connections to its stdout until it receives the string EOF.

package main

import (
	"fmt"
	"os"
	"strings"
	"strconv"
	"io"
	"net"
	"bufio"
)

func checkPrefix(prefixMap map[string]struct{}, prefix string, arg string) bool {
	if _, found := prefixMap[prefix]; found {
		fmt.Printf("prefix specified more then once: %s", arg)
		os.Exit(-1)
	}

	if strings.HasPrefix(arg, prefix) {
		prefixMap[prefix] = struct{}{}
		return true
	}

	return false
}

func main() {
	var outputStream io.Writer
	outputStream = os.Stdout
	seenPrefixes := make(map[string]struct{})
	exit_code := 0

	for _, arg := range os.Args[1:] {
		if checkPrefix(seenPrefixes, "stream=", arg) {
			switch arg {
			case "stream=stdout":
				outputStream = os.Stdout
			case "stream=stderr":
				outputStream = os.Stderr
			case "stream=both":
				outputStream = io.MultiWriter(os.Stdout, os.Stderr)
			default:
				fmt.Printf("unrecognized stream specification: %s", arg)
				os.Exit(-1)
			}
		} else if checkPrefix(seenPrefixes, "exit=", arg) {
			exit_code_str := arg[5:]
			var err error
			exit_code_conv, err := strconv.Atoi(exit_code_str)
			exit_code = exit_code_conv
			if err != nil {
				fmt.Printf("Exit code %s not an int!", exit_code_str)
				os.Exit(-1)
			}
		}
	}

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		fmt.Printf("Error starting tcp server: %v\n", err)
		os.Exit(-1)
	}
	defer l.Close()

	// Emit the address of the server
	fmt.Printf("%v\n",l.Addr())

	manageCh := make(chan struct{})

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				fmt.Printf("Error accepting connection: %v\n", err)
				os.Exit(-1)
			}
			go handleRequest(manageCh, outputStream, conn)
		}
	}()

	<- manageCh
	l.Close()

	os.Exit(exit_code)
}

// Handles incoming requests.
func handleRequest(manageCh chan<- struct{}, w io.Writer, conn net.Conn) {
	defer conn.Close()
	bio := bufio.NewScanner(conn)
	for bio.Scan() {
		if line := strings.TrimSuffix(bio.Text(), "\n"); line == "EOF" {
			// Request program close
			select {
				case manageCh <- struct{}{}:
					// Request sent.
				default:
					// Already closing
			}
			break
		}
		fmt.Fprintf(w, "%s\n", bio.Text())
	}
}