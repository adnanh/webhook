package network

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Agent struct {
	cfg        *AgentConfig
	dialer     websocket.Dialer
	services   map[string]Service
	sem        chan struct{}
	wg         sync.WaitGroup
	hooks      func() []HookInfo
	downstream *Hub
	webhook    http.Handler
	handlers   map[string]http.Handler
	info       AgentInfo
}

type AgentOptions struct {
	Version        string
	Temporary      bool
	ExpiresAt      *time.Time
	Hooks          func() []HookInfo
	Downstream     *Hub
	WebhookHandler http.Handler
	Handlers       map[string]http.Handler
}

func RunAgent(ctx context.Context, cfg *AgentConfig, options ...AgentOptions) error {
	a, err := newAgent(cfg)
	if err != nil {
		return err
	}
	if len(options) != 0 {
		a.hooks = options[0].Hooks
		a.downstream = options[0].Downstream
		a.webhook = options[0].WebhookHandler
		a.handlers = options[0].Handlers
		a.info.Version = options[0].Version
		a.info.Temporary = options[0].Temporary
		a.info.ExpiresAt = options[0].ExpiresAt
	}
	a.info.OS = runtime.GOOS
	a.info.Arch = runtime.GOARCH
	if a.info.Version == "" {
		a.info.Version = "unknown"
	}
	if cfg.WebhookOnlyViaHub && a.webhook == nil {
		return errors.New("webhook_only_via_hub requires an in-process webhook handler")
	}
	if a.hooks == nil && a.downstream != nil {
		if service, ok := a.services["webhook"]; ok && service.RelayTo != "" {
			id, _, _ := splitDestination(service.RelayTo)
			a.hooks = func() []HookInfo { return a.downstream.HooksForNode(id) }
		}
	}
	listeners := make([]net.Listener, 0, len(cfg.Forwards))
	for _, forward := range cfg.Forwards {
		listener, err := net.Listen("tcp", forward.Listen)
		if err != nil {
			for _, opened := range listeners {
				_ = opened.Close()
			}
			return fmt.Errorf("listen on %s: %w", forward.Listen, err)
		}
		listeners = append(listeners, listener)
	}
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
		a.wg.Wait()
	}()
	for i, listener := range listeners {
		forward := cfg.Forwards[i]
		a.wg.Add(1)
		go a.accept(ctx, listener, forward)
		log.Printf("network forward %s -> %s", listener.Addr(), forward.To)
	}
	go func() {
		<-ctx.Done()
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	backoff := time.Second
	for ctx.Err() == nil {
		connectedAt := time.Now()
		err := a.control(ctx)
		if time.Since(connectedAt) >= time.Minute {
			backoff = time.Second
		}
		if ctx.Err() != nil {
			break
		}
		log.Printf("network control disconnected: %v", err)
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		select {
		case <-ctx.Done():
		case <-time.After(backoff + jitter):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
	return nil
}

func newAgent(cfg *AgentConfig) (*Agent, error) {
	a := &Agent{
		cfg:      cfg,
		services: make(map[string]Service),
		sem:      make(chan struct{}, cfg.MaxConnections),
		dialer: websocket.Dialer{
			Proxy:            http.ProxyFromEnvironment,
			HandshakeTimeout: 10 * time.Second,
			TLSClientConfig:  &tls.Config{MinVersion: tls.VersionTLS12},
			ReadBufferSize:   4096,
			WriteBufferSize:  4096,
		},
	}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca_file: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("ca_file contains no certificates")
		}
		a.dialer.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	for _, service := range cfg.Services {
		a.services[service.Name] = service
	}
	return a, nil
}

func (a *Agent) openWebSocket(ctx context.Context, path, role, destination, session string) (*websocket.Conn, error) {
	nonce, err := newNonce()
	if err != nil {
		return nil, fmt.Errorf("generate connection nonce: %w", err)
	}
	head := http.Header{}
	head.Set(nodeHeader, a.cfg.ID)
	head.Set(nonceHeader, base64.RawURLEncoding.EncodeToString(nonce))
	if role != "" {
		head.Set(roleHeader, role)
	}
	if destination != "" {
		head.Set(toHeader, destination)
	}
	if session != "" {
		head.Set(sessionHeader, session)
	}
	key := sha256.Sum256([]byte(a.cfg.token))
	head.Set("Authorization", "Network-HMAC "+connectionProof(key, "webhook-network-agent-proof-v1", nonce, a.cfg.ID, path, role, destination, session))
	conn, response, err := a.dialer.DialContext(ctx, a.cfg.Server+path, head)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil && response != nil {
		return nil, fmt.Errorf("network hub returned HTTP %d: %w", response.StatusCode, err)
	}
	if err != nil {
		return nil, err
	}
	expected := connectionProof(key, "webhook-network-hub-proof-v1", nonce, a.cfg.ID, path, role, destination, session)
	if response == nil || subtle.ConstantTimeCompare([]byte(response.Header.Get(proofHeader)), []byte(expected)) != 1 {
		_ = conn.Close()
		return nil, errors.New("network hub authentication failed")
	}
	return conn, nil
}

func (a *Agent) control(ctx context.Context) error {
	conn, err := a.openWebSocket(ctx, ControlPath, "", "", "")
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()
	services := make([]string, 0, len(a.cfg.Services))
	for _, service := range a.cfg.Services {
		services = append(services, service.Name)
	}
	var inventory []HookInfo
	if a.hooks != nil {
		inventory = a.hooks()
	}
	if err := conn.WriteJSON(controlMessage{Info: &a.info, Type: "register", Services: services, Hooks: inventory}); err != nil {
		return err
	}
	if a.hooks != nil {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			last := inventory
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					next := a.hooks()
					if reflect.DeepEqual(next, last) {
						continue
					}
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteJSON(controlMessage{Type: "inventory", Hooks: next}); err != nil {
						_ = conn.Close()
						return
					}
					last = next
				}
			}
		}()
	}
	log.Printf("network node %s connected to hub", a.cfg.ID)
	conn.SetReadLimit(controlLimit)
	_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	})
	go pingLoop(conn, stop)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var message controlMessage
		if err := json.Unmarshal(data, &message); err != nil || message.Type != "open" || message.Session == "" || !validName(message.Service) {
			return errors.New("invalid control message")
		}
		target, exists := a.services[message.Service]
		if !exists {
			continue
		}
		if !a.acquire() {
			continue
		}
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			defer a.release()
			a.handleTarget(ctx, message.Session, target, message.Hops)
		}()
	}
}

func (a *Agent) acquire() bool {
	select {
	case a.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (a *Agent) release() { <-a.sem }

func (a *Agent) accept(ctx context.Context, listener net.Listener, forward Forward) {
	defer a.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		if !a.acquire() {
			_ = conn.Close()
			continue
		}
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			defer a.release()
			a.handleSource(ctx, conn, forward.To)
		}()
	}
}

func (a *Agent) handleSource(ctx context.Context, conn net.Conn, destination string) {
	defer conn.Close()
	ws, err := a.openWebSocket(ctx, DataPath, "source", destination, "")
	if err != nil {
		log.Printf("network forward %s failed: %v", destination, err)
		return
	}
	a.bridge(ctx, conn, ws)
}

func (a *Agent) handleTarget(ctx context.Context, session string, service Service, hops int) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	var conn net.Conn
	var err error
	if service.RelayTo != "" {
		if a.downstream == nil || hops <= 1 {
			cancel()
			return
		}
		id, name, _ := splitDestination(service.RelayTo)
		conn, err = a.downstream.openPipe(dialCtx, id, name, hops-1)
	} else if handler := a.handlers[service.Name]; handler != nil {
		conn = serveWebhookInProcess(handler)
	} else if service.Name == "webhook" && service.Target == "" && a.webhook != nil {
		conn = serveWebhookInProcess(a.webhook)
	} else {
		conn, err = (&net.Dialer{}).DialContext(dialCtx, "tcp", service.Target)
	}
	cancel()
	if err != nil {
		log.Printf("network target %s failed: %v", service.Name, err)
		return
	}
	defer conn.Close()
	ws, err := a.openWebSocket(ctx, DataPath, "target", "", session)
	if err != nil {
		log.Printf("network target tunnel failed: %v", err)
		return
	}
	a.bridge(ctx, conn, ws)
}

func (a *Agent) bridge(ctx context.Context, conn net.Conn, ws *websocket.Conn) {
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
			_ = ws.Close()
		case <-stop:
		}
	}()
	bridgeTCP(conn, ws)
	close(stop)
}
