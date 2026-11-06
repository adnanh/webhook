package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// configFile holds the path to the YAML configuration file.
// It is extracted from os.Args before flag.Parse() runs.
var configFile string

// An inline network section is parsed after CLI flags so -network-config can
// override it in the same way as other command-line options.
var inlineNetworkConfig []byte
var inlineNetworkBase string

// loadConfigFile reads a YAML configuration file and sets the corresponding
// flags. Flags set via the command line take precedence over the config file
// because this function is called before flag.Parse().
//
// Supported YAML structure:
//
//	ip: 0.0.0.0
//	port: 9000
//	nopanic: true
//	hooks:
//	  - /etc/webhook/hooks.yaml
//	header:
//	  - "X-Custom=value"
func loadConfigFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	// Use a generic map so we can handle both scalar and list values.
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file %q: %w", path, err)
	}
	if _, inline := cfg["network"]; inline {
		if _, external := cfg["network-config"]; external {
			return fmt.Errorf("config file %q cannot set both network and network-config", path)
		}
	}

	for key, val := range cfg {
		if key == "network" {
			networkYAML, err := yaml.Marshal(val)
			if err != nil {
				return fmt.Errorf("failed to parse network in config file %q: %w", path, err)
			}
			inlineNetworkConfig = networkYAML
			inlineNetworkBase = filepath.Dir(path)
			continue
		}
		if key == "network-config" {
			name, ok := val.(string)
			if !ok || name == "" {
				return fmt.Errorf("network-config in config file %q must be a path", path)
			}
			if !filepath.IsAbs(name) {
				val = filepath.Join(filepath.Dir(path), name)
			}
		}
		f := flag.CommandLine.Lookup(key)
		if f == nil {
			return fmt.Errorf("unknown option %q in config file %q", key, path)
		}

		switch v := val.(type) {
		case []interface{}:
			// List values: call flag.Set for each element (supports -hooks, -header, etc.)
			for _, item := range v {
				s := fmt.Sprintf("%v", item)
				if err := flag.CommandLine.Set(key, s); err != nil {
					return fmt.Errorf("error setting flag %q to %q: %w", key, s, err)
				}
			}
		case bool:
			s := "false"
			if v {
				s = "true"
			}
			if err := flag.CommandLine.Set(key, s); err != nil {
				return fmt.Errorf("error setting flag %q to %q: %w", key, s, err)
			}
		default:
			s := fmt.Sprintf("%v", v)
			if err := flag.CommandLine.Set(key, s); err != nil {
				return fmt.Errorf("error setting flag %q to %q: %w", key, s, err)
			}
		}
	}

	return nil
}

// extractConfigFlag scans os.Args for -config / -c / --config and returns the
// path value. It also removes those arguments from os.Args so that flag.Parse()
// does not see them (since -config is not a registered flag).
func extractConfigFlag() string {
	var path string
	var newArgs []string

	args := os.Args[1:] // skip program name
	newArgs = append(newArgs, os.Args[0])

	for i := 0; i < len(args); i++ {
		arg := args[i]

		var matched bool
		var value string

		switch {
		case arg == "-c" || arg == "-config" || arg == "--config":
			matched = true
			if i+1 < len(args) {
				i++
				value = args[i]
			}
		case strings.HasPrefix(arg, "-c="):
			matched = true
			value = strings.TrimPrefix(arg, "-c=")
		case strings.HasPrefix(arg, "-config="):
			matched = true
			value = strings.TrimPrefix(arg, "-config=")
		case strings.HasPrefix(arg, "--config="):
			matched = true
			value = strings.TrimPrefix(arg, "--config=")
		}

		if matched {
			path = value
		} else {
			newArgs = append(newArgs, arg)
		}
	}

	os.Args = newArgs
	return path
}
