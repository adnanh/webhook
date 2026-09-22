package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSelfSignedCert(t *testing.T, certPath, keyPath string, serial int64) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "webhook-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	mtime := time.Unix(1_000_000_000+serial, 0)
	if err := os.Chtimes(certPath, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(keyPath, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func certSerial(t *testing.T, cert *tls.Certificate) int64 {
	t.Helper()

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}

	return leaf.SerialNumber.Int64()
}

func TestCertReloader(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	writeSelfSignedCert(t, certPath, keyPath, 1)

	reloader, err := newCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := reloader.getCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := certSerial(t, cert); got != 1 {
		t.Fatalf("serial = %d, want 1", got)
	}

	writeSelfSignedCert(t, certPath, keyPath, 2)

	cert, err = reloader.getCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := certSerial(t, cert); got != 2 {
		t.Fatalf("serial after renewal = %d, want 2", got)
	}

	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}

	cert, err = reloader.getCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := certSerial(t, cert); got != 2 {
		t.Fatalf("serial with missing key = %d, want previous certificate 2", got)
	}
}
