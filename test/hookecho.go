// Hook Echo is a simply utility used for testing the Webhook package.

package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Printf("arg: %s\n", strings.Join(os.Args[1:], " "))
	}

	var env []string
	for _, v := range os.Environ() {
		if strings.HasPrefix(v, "HOOK_") {
			env = append(env, v)
		}
	}
	sort.Strings(env)

	if len(env) > 0 {
		fmt.Printf("env: %s\n", strings.Join(env, " "))
	}

	for _, arg := range os.Args[1:] {
		switch {
		case strings.HasPrefix(arg, "cat-env-file="):
			key := strings.TrimPrefix(arg, "cat-env-file=")
			path := os.Getenv(key)
			if path == "" {
				fmt.Printf("File env %s is not set!", key)
				os.Exit(-1)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("Failed to read %s (%s): %v", key, path, err)
				os.Exit(-1)
			}
			fmt.Printf("file: %s=%s\n", key, string(content))

		case strings.HasPrefix(arg, "sleep="):
			sleepFor := strings.TrimPrefix(arg, "sleep=")
			duration, err := time.ParseDuration(sleepFor)
			if err != nil {
				fmt.Printf("Sleep duration %s is invalid!", sleepFor)
				os.Exit(-1)
			}
			time.Sleep(duration)
			fmt.Printf("slept: %s\n", duration)

		case strings.HasPrefix(arg, "exit="):
			exit_code_str := arg[5:]
			exit_code, err := strconv.Atoi(exit_code_str)
			if err != nil {
				fmt.Printf("Exit code %s not an int!", exit_code_str)
				os.Exit(-1)
			}
			os.Exit(exit_code)
		}
	}
}
