// Hook Echo is a simply utility used for testing the Webhook package.

package main

import (
	"fmt"
	"os"
	"strings"
	"strconv"
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

	if len(env) > 0 {
		fmt.Printf("env: %s\n", strings.Join(env, " "))
	}

	if (len(os.Args) > 1) && (strings.HasPrefix(os.Args[1], "exit=")) {
		exit_code_str := os.Args[1][5:]
		exit_code, err := strconv.Atoi(exit_code_str)
		if err != nil {
			fmt.Printf("Exit code %s not an int!", exit_code_str)
			os.Exit(-1)
		}
		os.Exit(exit_code)
	}
}
