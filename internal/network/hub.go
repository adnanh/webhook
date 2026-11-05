package network

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	nodeHeader    = "X-Webhook-Network-Node"
	roleHeader    = "X-Webhook-Network-Role"
	toHeader      = "X-Webhook-Network-To"
	sessionHeader = "X-Webhook-Network-Session"
	controlLimit  = 1 << 20
	openTimeout   = 10 * time.Second
	maxHops       = 4
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

type controlMessage struct {
	Info     *AgentInfo `json:"info,omitempty"`
	Type     string     `json:"type"`
	Services []string   `json:"services,omitempty"`
	Hooks    []HookInfo `json:"hooks,omitempty"`
	Session  string     `json:"session,omitempty"`
	Service  string     `json:"service,omitempty"`
	Hops     int        `json:"hops,omitempty"`
}

// HookInfo contains only the metadata needed for early filtering at the hub.
// The agent remains responsible for evaluating the complete hook rules.
type HookInfo struct {
	ID         string   `json:"id"`
	Methods    []string `json:"methods,omitempty"`
	IPRequired []string `json:"ip_required,omitempty"`
	AllowIPs   []string `json:"allow_ips,omitempty"`
	DenyIPs    []string `json:"deny_ips,omitempty"`
}

type controlConn struct {
	conn     *websocket.Conn
	services map[string]bool
	hooks    map[string]HookInfo
	writeMu  sync.Mutex
}

func (c *controlConn) write(message controlMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteJSON(message)
}

type pendingSession struct {
	sourceID string
	source   *websocket.Conn
	pipe     net.Conn
	targetID string
	ready    chan *websocket.Conn
	paired   bool
	target   *websocket.Conn
}

type Hub struct {
	states         map[string]NodeStatus
	replays        map[string]time.Time
	nextPrune      time.Time
	requestTimeout time.Duration
	mu             sync.Mutex
	nodes          map[string]NodePolicy
	controls       map[string]*controlConn
	sessions       map[string]*pendingSession
	max            int
	routes         map[string]HookRoute
	closed         bool
}

func NewHub(cfg *HubConfig) *Hub {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	h := &Hub{
		states:         make(map[string]NodeStatus),
		replays:        make(map[string]time.Time),
		requestTimeout: timeout,
		nodes:          make(map[string]NodePolicy),
		controls:       make(map[string]*controlConn),
		sessions:       make(map[string]*pendingSession),
		max:            cfg.MaxSessions,
		routes:         make(map[string]HookRoute),
	}
	for _, n := range cfg.Nodes {
		h.nodes[n.ID] = n
		h.states[n.ID] = NodeStatus{ID: n.ID, Status: "never_connected", AgentInfo: AgentInfo{Version: "unknown"}}
	}
	for _, route := range cfg.Routes {
		h.routes[route.Path] = route
	}
	return h
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for _, c := range h.controls {
		_ = c.conn.Close()
	}
	for _, s := range h.sessions {
		if s.source != nil {
			_ = s.source.Close()
		}
		if s.pipe != nil {
			_ = s.pipe.Close()
		}
		if s.target != nil {
			_ = s.target.Close()
		}
	}
}

func (h *Hub) Handles(path string) bool {
	return path == ControlPath || path == DataPath
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get(nodeHeader) == "" || r.Header.Get("Authorization") == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	nonce, err := base64.RawURLEncoding.DecodeString(r.Header.Get(nonceHeader))
	if err != nil || len(nonce) != 32 {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}
	node, ok := h.authenticate(r, nonce)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	proof := http.Header{}
	proof.Set(proofHeader, requestProof(h.nodes[node].tokenHash, "webhook-network-hub-proof-v1", nonce, r))
	switch r.URL.Path {
	case ControlPath:
		h.handleControl(w, r, node, proof)
	case DataPath:
		switch r.Header.Get(roleHeader) {
		case "source":
			h.handleSource(w, r, node, proof)
		case "target":
			h.handleTarget(w, r, node, proof)
		default:
			http.Error(w, "invalid role", http.StatusBadRequest)
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *Hub) authenticate(r *http.Request, nonce []byte) (string, bool) {
	id := r.Header.Get(nodeHeader)
	policy, exists := h.nodes[id]
	value := r.Header.Get("Authorization")
	const prefix = "Network-HMAC "
	if !exists || len(value) < len(prefix) || value[:len(prefix)] != prefix {
		return "", false
	}
	proof, err := base64.RawURLEncoding.DecodeString(value[len(prefix):])
	if err != nil || len(proof) != sha256.Size {
		return "", false
	}
	expected, _ := base64.RawURLEncoding.DecodeString(requestProof(policy.tokenHash, "webhook-network-agent-proof-v1", nonce, r))
	if subtle.ConstantTimeCompare(proof, expected) != 1 {
		return "", false
	}
	return id, h.consumeNonce(id, nonce)
}

func (h *Hub) handleControl(w http.ResponseWriter, r *http.Request, id string, proof http.Header) {
	conn, err := upgrader.Upgrade(w, r, proof)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(controlLimit)
	_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	})
	var register controlMessage
	if err := conn.ReadJSON(&register); err != nil || register.Type != "register" {
		return
	}
	services := make(map[string]bool)
	for _, service := range register.Services {
		if !contains(h.nodes[id].Services, service) || services[service] {
			return
		}
		services[service] = true
	}
	hooks, valid := validateInventory(register.Hooks)
	if !valid {
		return
	}
	info := AgentInfo{Version: "unknown"}
	if register.Info != nil {
		info = *register.Info
		if !validAgentInfo(info) {
			return
		}
		if info.Version == "" {
			info.Version = "unknown"
		}
	}
	control := &controlConn{conn: conn, services: services, hooks: hooks}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	old := h.controls[id]
	h.controls[id] = control
	now := time.Now().UTC()
	h.states[id] = NodeStatus{ID: id, Status: "online", AgentInfo: info, ConnectedAt: &now, LastSeenAt: &now, PeerAddress: r.RemoteAddr}
	h.mu.Unlock()
	if old != nil {
		_ = old.conn.Close()
	}
	defer func() {
		h.mu.Lock()
		if h.controls[id] == control {
			delete(h.controls, id)
			state := h.states[id]
			now := time.Now().UTC()
			state.Status = "offline"
			state.DisconnectedAt = &now
			h.states[id] = state
		}
		h.mu.Unlock()
	}()
	conn.SetPongHandler(func(string) error {
		h.markSeen(id, control)
		return conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	})
	log.Printf("network node %s connected", id)
	defer log.Printf("network node %s disconnected", id)
	stop := make(chan struct{})
	defer close(stop)
	go pingLoop(conn, stop)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var message controlMessage
		if json.Unmarshal(data, &message) != nil {
			return
		}
		h.markSeen(id, control)
		if message.Type == "inventory" {
			hooks, valid := validateInventory(message.Hooks)
			if !valid {
				return
			}
			h.mu.Lock()
			if h.controls[id] == control {
				control.hooks = hooks
			}
			h.mu.Unlock()
		} else if message.Type != "ping" {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	}
}

func (h *Hub) handleSource(w http.ResponseWriter, r *http.Request, id string, proof http.Header) {
	to := r.Header.Get(toHeader)
	targetID, service, valid := splitDestination(to)
	if !valid || service == "webhook" || !contains(h.nodes[id].Allow, to) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h.mu.Lock()
	control := h.controls[targetID]
	available := !h.closed && control != nil && control.services[service]
	full := len(h.sessions) >= h.max
	h.mu.Unlock()
	if !available {
		http.Error(w, "target unavailable", http.StatusServiceUnavailable)
		return
	}
	if full {
		http.Error(w, "too many sessions", http.StatusTooManyRequests)
		return
	}
	conn, err := upgrader.Upgrade(w, r, proof)
	if err != nil {
		return
	}
	defer conn.Close()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return
	}
	sessionID := hex.EncodeToString(random[:])
	session := &pendingSession{sourceID: id, source: conn, targetID: targetID, ready: make(chan *websocket.Conn, 1)}
	h.mu.Lock()
	if h.closed || len(h.sessions) >= h.max || h.controls[targetID] != control {
		h.mu.Unlock()
		return
	}
	h.sessions[sessionID] = session
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		if session.target != nil {
			_ = session.target.Close()
		}
		h.mu.Unlock()
	}()
	if err := control.write(controlMessage{Type: "open", Session: sessionID, Service: service, Hops: maxHops}); err != nil {
		return
	}
	var target *websocket.Conn
	select {
	case target = <-session.ready:
	case <-time.After(openTimeout):
		return
	}
	defer target.Close()
	bridgeWebSockets(conn, target)
}

func (h *Hub) handleTarget(w http.ResponseWriter, r *http.Request, id string, proof http.Header) {
	sessionID := r.Header.Get(sessionHeader)
	h.mu.Lock()
	session := h.sessions[sessionID]
	valid := session != nil && session.targetID == id
	h.mu.Unlock()
	if !valid {
		http.Error(w, "unknown session", http.StatusForbidden)
		return
	}
	conn, err := upgrader.Upgrade(w, r, proof)
	if err != nil {
		return
	}
	h.mu.Lock()
	if h.sessions[sessionID] != session || session.paired {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	session.paired = true
	session.target = conn
	session.ready <- conn
	h.mu.Unlock()
}
