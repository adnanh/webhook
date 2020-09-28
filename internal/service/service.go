// Package service manages the webhook HTTP service.
package service

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/adnanh/webhook/internal/pidfile"
	"github.com/adnanh/webhook/internal/service/security"

	"github.com/gorilla/mux"
)

// Service is the webhook HTTP service.
type Service struct {
	// Address is the listener address for the service (e.g. "127.0.0.1:9000")
	Address string

	// TLS settings
	enableTLS     bool
	tlsCiphers    []uint16
	tlsMinVersion uint16
	kpr           *security.KeyPairReloader

	// Future TLS settings to consider:
	// - tlsMaxVersion
	// - configurable TLS curves
	// - modern and intermediate helpers that follows Mozilla guidelines
	// - ca root and intermediate certs

	listener net.Listener
	server   *http.Server

	pidFile *pidfile.PIDFile

	// Hooks map[string]hook.Hooks
}

// New creates a new webhook HTTP service for the given address and port.
func New(ip string, port int) *Service {
	return &Service{
		Address:       fmt.Sprintf("%s:%d", ip, port),
		server:        &http.Server{},
		tlsMinVersion: tls.VersionTLS12,
	}
}

// Listen announces the TCP service on the local network.
//
// To enable TLS, ensure that SetTLSEnabled is called prior to Listen.
//
// After calling Listen, Serve must be called to begin serving HTTP requests.
// The steps are separated so that we can drop privileges, if necessary, after
// opening the listening port.
func (s *Service) Listen() error {
	ln, err := net.Listen("tcp", s.Address)
	if err != nil {
		return err
	}

	if !s.enableTLS {
		s.listener = ln
		return nil
	}

	if s.kpr == nil {
		panic("Listen called with TLS enabled but KPR is nil")
	}

	c := &tls.Config{
		GetCertificate:           s.kpr.GetCertificateFunc(),
		CipherSuites:             s.tlsCiphers,
		CurvePreferences:         security.GetTLSCurves(""),
		MinVersion:               s.tlsMinVersion,
		PreferServerCipherSuites: true,
	}

	s.listener = tls.NewListener(ln, c)

	return nil
}

// Serve begins accepting incoming HTTP connections.
func (s *Service) Serve() error {
	s.server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler)) // disable http/2

	if s.listener == nil {
		err := s.Listen()
		if err != nil {
			return err
		}
	}

	defer s.listener.Close()
	return s.server.Serve(s.listener)
}

// SetHTTPHandler sets the underly HTTP server Handler.
func (s *Service) SetHTTPHandler(r *mux.Router) {
	s.server.Handler = r
}

// SetTLSCiphers sets the supported TLS ciphers.
func (s *Service) SetTLSCiphers(suites string) {
	s.tlsCiphers = security.GetTLSCipherSuites(suites)
}

// SetTLSEnabled enables TLS for the service. Must be called prior to Listen.
func (s *Service) SetTLSEnabled() {
	s.enableTLS = true
}

// SetTLSKeyPair sets the TLS key pair for the service.
func (s *Service) SetTLSKeyPair(certPath, keyPath string) error {
	if certPath == "" {
		return fmt.Errorf("error: certificate required for TLS")
	}

	if keyPath == "" {
		return fmt.Errorf("error: key required for TLS")
	}

	var err error

	s.kpr, err = security.NewKeyPairReloader(certPath, keyPath)
	if err != nil {
		return err
	}

	return nil
}

// SetTLSMinVersion sets the minimum support TLS version, such as "v1.3".
func (s *Service) SetTLSMinVersion(ver string) (err error) {
	s.tlsMinVersion, err = security.GetTLSVersion(ver)
	return err
}

// CreatePIDFile creates a new PID file at path p.
func (s *Service) CreatePIDFile(p string) (err error) {
	s.pidFile, err = pidfile.New(p)
	return err
}

// DeletePIDFile deletes a previously created PID file.
func (s *Service) DeletePIDFile() error {
	if s.pidFile != nil {
		return s.pidFile.Remove()
	}
	return nil
}
