// Package security provides HTTP security management help to the webhook
// service.
package security

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
)

// KeyPairReloader contains the active TLS certificate.  It can be used with
// the tls.Config.GetCertificate property to support live updating of the
// certificate.
type KeyPairReloader struct {
	certMu   sync.RWMutex
	cert     *tls.Certificate
	certPath string
	keyPath  string
}

// NewKeyPairReloader creates a new KeyPairReloader given the certificate and
// key path.
func NewKeyPairReloader(certPath, keyPath string) (*KeyPairReloader, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	res := &KeyPairReloader{
		cert:     &cert,
		certPath: certPath,
		keyPath:  keyPath,
	}

	return res, nil
}

// GetCertificateFunc provides a function for tls.Config.GetCertificate.
func (kpr *KeyPairReloader) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		kpr.certMu.RLock()
		defer kpr.certMu.RUnlock()
		return kpr.cert, nil
	}
}

// WriteTLSSupportedCipherStrings writes a list of ciphers to w.  The list is
// all supported TLS ciphers based upon min.
func WriteTLSSupportedCipherStrings(w io.Writer, min string) error {
	m, err := GetTLSVersion(min)
	if err != nil {
		return err
	}

	for _, c := range tls.CipherSuites() {
		var found bool

		for _, v := range c.SupportedVersions {
			if v >= m {
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

// GetTLSVersion converts a TLS version string, v, (e.g. "v1.3") into a TLS
// version ID.
func GetTLSVersion(v string) (uint16, error) {
	switch v {
	case "1.3", "v1.3", "tls1.3":
		return tls.VersionTLS13, nil
	case "1.2", "v1.2", "tls1.2", "":
		return tls.VersionTLS12, nil
	case "1.1", "v1.1", "tls1.1":
		return tls.VersionTLS11, nil
	case "1.0", "v1.0", "tls1.0":
		return tls.VersionTLS10, nil
	default:
		return 0, fmt.Errorf("error: unknown TLS version: %s", v)
	}
}

// GetTLSCipherSuites converts a comma separated list of cipher suites into a
// slice of TLS cipher suite IDs.
func GetTLSCipherSuites(v string) []uint16 {
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

// GetTLSCurves converts a comma separated list of curves into a
// slice of TLS curve IDs.
func GetTLSCurves(v string) []tls.CurveID {
	supported := []tls.CurveID{
		tls.CurveP256,
		tls.CurveP384,
		tls.CurveP521,
		tls.X25519,
	}

	if v == "" {
		return supported
	}

	var found bool
	txts := strings.Split(v, ",")
	res := make([]tls.CurveID, len(txts))

	for _, want := range txts {
		found = false

		for _, c := range supported {
			if want == c.String() {
				res = append(res, c)
				found = true
			}
		}

		if !found {
			log.Fatalln("error: unknown TLS curve:", want)
		}
	}

	return res
}
