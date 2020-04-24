package main

import (
	"crypto/tls"
	"io"
	"log"
	"strings"
)

func writeTLSSupportedCipherStrings(w io.Writer, min uint16) error {
	for _, c := range tls.CipherSuites() {
		var found bool

		for _, v := range c.SupportedVersions {
			if v >= min {
				found = true
			}
		}

		if !found {
			continue
		}

		_, err := w.Write([]byte(c.Name + "\n"))
		if err != nil {
			return err
		}
	}

	return nil
}

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
	supported := tls.CipherSuites()

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
