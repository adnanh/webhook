package network

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	mathrand "math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const ClientIPHeader = "X-Webhook-Network-Client-IP"
const OriginalURIHeader = "X-Webhook-Network-Original-URI"

type RequestOrigin struct {
	ClientIP string
	Scheme   string
}

func validateInventory(items []HookInfo) (map[string]HookInfo, bool) {
	if len(items) > 1024 {
		return nil, false
	}
	result := make(map[string]HookInfo, len(items))
	for _, item := range items {
		if !validName(item.ID) || len(item.Methods) > 16 || len(item.IPRequired) > 16 || len(item.AllowIPs) > 64 || len(item.DenyIPs) > 64 || (len(item.AllowIPs) > 0 && len(item.DenyIPs) > 0) {
			return nil, false
		}
		if _, exists := result[item.ID]; exists {
			return nil, false
		}
		for _, method := range item.Methods {
			if method == "" || strings.ToUpper(method) != method || strings.ContainsAny(method, " \t\r\n") {
				return nil, false
			}
		}
		for _, rangeText := range item.IPRequired {
			parts := strings.FieldsFunc(rangeText, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
			if len(parts) == 0 {
				return nil, false
			}
			if _, err := parseNetworks(parts); err != nil {
				return nil, false
			}
		}
		if _, err := parseNetworks(item.AllowIPs); err != nil {
			return nil, false
		}
		if _, err := parseNetworks(item.DenyIPs); err != nil {
			return nil, false
		}
		result[item.ID] = item
	}
	return result, true
}

func (h *Hub) Routes() []HookRoute {
	routes := make([]HookRoute, 0, len(h.routes))
	for _, route := range h.routes {
		routes = append(routes, route)
	}
	return routes
}

func includesIP(networks []*net.IPNet, ip net.IP) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func hookAllowsIP(info HookInfo, ip net.IP) bool {
	allow, _ := parseNetworks(info.AllowIPs)
	deny, _ := parseNetworks(info.DenyIPs)
	if len(allow) != 0 && !includesIP(allow, ip) || includesIP(deny, ip) {
		return false
	}
	for _, rangeText := range info.IPRequired {
		parts := strings.FieldsFunc(rangeText, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
		networks, err := parseNetworks(parts)
		if err != nil || !includesIP(networks, ip) {
			return false
		}
	}
	return true
}

// ServeHook routes one public hook URL to one randomly selected eligible agent.
// Every complete hook rule is still evaluated by the selected agent.
func (h *Hub) ServeHook(w http.ResponseWriter, r *http.Request, origin RequestOrigin, maxBodySize int64) {
	ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline.Add(time.Second))
	defer controller.SetReadDeadline(time.Time{})
	defer controller.SetWriteDeadline(time.Time{})
	route, exists := h.routes[r.URL.Path]
	if !exists {
		http.NotFound(w, r)
		return
	}
	ip := net.ParseIP(origin.ClientIP)
	if ip == nil || (len(route.allowNets) != 0 && !includesIP(route.allowNets, ip)) || includesIP(route.denyNets, ip) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}
	type candidate struct {
		id string
	}
	var eligible []candidate
	available, methodDenied, ipDenied := false, false, false
	h.mu.Lock()
	for _, id := range route.Agents {
		control := h.controls[id]
		if control == nil || !control.services["webhook"] {
			continue
		}
		info, hasHook := control.hooks[route.Hook]
		if !hasHook {
			continue
		}
		available = true
		if len(info.Methods) != 0 && !contains(info.Methods, r.Method) {
			methodDenied = true
			continue
		}
		if !hookAllowsIP(info, ip) {
			ipDenied = true
			continue
		}
		eligible = append(eligible, candidate{id: id})
	}
	h.mu.Unlock()
	if len(eligible) == 0 {
		switch {
		case ipDenied:
			http.Error(w, "access denied", http.StatusForbidden)
		case methodDenied:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case !available:
			http.Error(w, "hook unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	selected := eligible[mathrand.Intn(len(eligible))].id
	if maxBodySize > 0 {
		if r.ContentLength > maxBodySize {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	}
	backendPath := h.nodes[selected].HookPrefix + "/" + route.Hook
	out := r.Clone(ctx)
	out.URL.Scheme = "http"
	out.URL.Host = "network.internal"
	out.URL.Path = backendPath
	out.URL.RawPath = ""
	out.RequestURI = ""
	out.Host = r.Host
	out.Header = r.Header.Clone()
	removeHopHeaders(out.Header)
	for _, name := range []string{ClientIPHeader, OriginalURIHeader, "Forwarded", "X-Forwarded-Port", "X-Forwarded-Prefix", "X-Forwarded-Server", "X-Forwarded-Ssl"} {
		out.Header.Del(name)
	}
	out.Header.Set(ClientIPHeader, origin.ClientIP)
	out.Header.Set(OriginalURIHeader, r.URL.RequestURI())
	out.Header.Set("X-Forwarded-For", origin.ClientIP)
	out.Header.Set("X-Real-IP", origin.ClientIP)
	scheme := origin.Scheme
	if scheme != "https" {
		scheme = "http"
	}
	out.Header.Set("X-Forwarded-Proto", scheme)
	out.Header.Set("X-Forwarded-Host", r.Host)
	out.Close = true
	transport := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return h.openPipe(ctx, selected, "webhook", maxHops)
		},
	}
	defer transport.CloseIdleConnections()
	response, err := transport.RoundTrip(out)
	if err != nil {
		var bodyErr *http.MaxBytesError
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || !time.Now().Before(deadline) {
			http.Error(w, "hook request timed out", http.StatusGatewayTimeout)
		} else if errors.As(err, &bodyErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "hook target unavailable", http.StatusBadGateway)
		}
		return
	}
	defer response.Body.Close()
	removeHopHeaders(response.Header)
	removeHopHeaders(response.Trailer)
	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	for key := range response.Trailer {
		w.Header().Add("Trailer", key)
	}
	w.WriteHeader(response.StatusCode)
	if _, err := io.Copy(w, response.Body); err != nil {
		// A truncated response must not look like a successful complete body.
		panic(http.ErrAbortHandler)
	}
	removeHopHeaders(response.Trailer)
	for key, values := range response.Trailer {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
}

func (h *Hub) removeSession(id string, session *pendingSession) {
	h.mu.Lock()
	if h.sessions[id] == session {
		delete(h.sessions, id)
	}
	if session.target != nil {
		_ = session.target.Close()
	}
	h.mu.Unlock()
}

func (h *Hub) openPipe(ctx context.Context, targetID, service string, hops int) (net.Conn, error) {
	if hops < 1 {
		return nil, errors.New("network relay hop limit reached")
	}
	h.mu.Lock()
	control := h.controls[targetID]
	if h.closed || control == nil || !control.services[service] || len(h.sessions) >= h.max {
		h.mu.Unlock()
		return nil, errors.New("target unavailable")
	}
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		h.mu.Unlock()
		return nil, err
	}
	id := hex.EncodeToString(randomBytes[:])
	session := &pendingSession{targetID: targetID, ready: make(chan *websocket.Conn, 1)}
	h.sessions[id] = session
	h.mu.Unlock()
	if err := control.write(controlMessage{Type: "open", Session: id, Service: service, Hops: hops}); err != nil {
		h.removeSession(id, session)
		return nil, err
	}
	var target *websocket.Conn
	select {
	case target = <-session.ready:
	case <-time.After(openTimeout):
		h.removeSession(id, session)
		return nil, errors.New("target connection timed out")
	case <-ctx.Done():
		h.removeSession(id, session)
		return nil, ctx.Err()
	}
	client, server := pipePair()
	h.mu.Lock()
	if h.closed || h.sessions[id] != session {
		h.mu.Unlock()
		_ = client.Close()
		_ = server.Close()
		_ = target.Close()
		return nil, errors.New("hub closed")
	}
	session.pipe = server
	h.mu.Unlock()
	go func() {
		bridgeTCP(server, target)
		h.removeSession(id, session)
	}()
	return client, nil
}

// DialService connects this hub to a service registered by a downstream node.
func (h *Hub) DialService(ctx context.Context, node, service string) (net.Conn, error) {
	if !validName(node) || !validName(service) || !contains(h.nodes[node].Services, service) {
		return nil, errors.New("unknown downstream service")
	}
	return h.openPipe(ctx, node, service, maxHops)
}

func (h *Hub) HooksForNode(id string) []HookInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	control := h.controls[id]
	if control == nil {
		return nil
	}
	result := make([]HookInfo, 0, len(control.hooks))
	for _, info := range control.hooks {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// Connection-nominated fields and hop-by-hop metadata belong to one HTTP hop.
func removeHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			header.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}
