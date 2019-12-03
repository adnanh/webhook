package main

import (
	"crypto/tls"
	"log"
	"strings"
)

// getTLSMinVersion converts a version string into a TLS version ID.
func getTLSMinVersion(v string) uint16 {
	switch v {
	case "1.0":
		return tls.VersionTLS10
	case "1.1":
		return tls.VersionTLS11
	case "1.2", "":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	default:
		log.Fatalln("error: unknown minimum TLS version:", v)
		return 0
	}
}

// getTLSCipherSuites converts a comma separated list of cipher suites into a
// slice of TLS cipher suite IDs.
func getTLSCipherSuites(v string) []uint16 {
	supported := CipherSuites()

	if v == "" {
		suites := make([]uint16, len(supported))

		for _, cs := range supported {
			suites = append(suites, cs.ID)
		}

		return suites
	}

	var found bool
	txts := strings.Split(v, ",")
	suites := make([]uint16, len(txts))

	for _, want := range txts {
		found = false

		for _, cs := range supported {
			if want == cs.Name {
				suites = append(suites, cs.ID)
				found = true
			}
		}

		if !found {
			log.Fatalln("error: unknown TLS cipher suite:", want)
		}
	}

	return suites
}
