// Command demo-hook prints selected inputs and performs no deployments.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: demo-hook echo|file|sleep|fail [arguments...]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "echo":
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Arguments []string `json:"arguments"`
			Trace     string   `json:"trace"`
		}{os.Args[2:], os.Getenv("DEMO_TRACE")})
	case "file":
		if len(os.Args) != 3 {
			os.Exit(2)
		}
		data, err := os.ReadFile(os.Getenv(os.Args[2]))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(string(data))
	case "sleep":
		if len(os.Args) != 3 {
			os.Exit(2)
		}
		duration, err := time.ParseDuration(os.Args[2])
		if err != nil || duration < 0 {
			os.Exit(2)
		}
		time.Sleep(duration)
		fmt.Println("completed")
	case "fail":
		fmt.Fprintln(os.Stderr, "demo failure")
		os.Exit(1)
	default:
		os.Exit(2)
	}
}
