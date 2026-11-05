package network

import (
	"sort"
	"strings"
	"time"
	"unicode"
)

// AgentInfo is self-reported by an authenticated node, not an attestation.
type AgentInfo struct {
	Version   string     `json:"version"`
	OS        string     `json:"os"`
	Arch      string     `json:"arch"`
	Temporary bool       `json:"temporary"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type NodeStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	AgentInfo
	PeerAddress    string     `json:"peer_address,omitempty"`
	ConnectedAt    *time.Time `json:"connected_at,omitempty"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
	LastSeenAt     *time.Time `json:"last_seen_at,omitempty"`
	Services       []string   `json:"services"`
	HookIDs        []string   `json:"hook_ids"`
	ActiveSessions int        `json:"active_sessions"`
}

type HubStatus struct {
	GeneratedAt    time.Time    `json:"generated_at"`
	ActiveSessions int          `json:"active_sessions"`
	MaxSessions    int          `json:"max_sessions"`
	Nodes          []NodeStatus `json:"nodes"`
}

func validAgentInfo(info AgentInfo) bool {
	for _, value := range []string{info.Version, info.OS, info.Arch} {
		if len(value) > 128 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return false
		}
	}
	return true
}

func (h *Hub) markSeen(id string, control *controlConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.controls[id] != control {
		return
	}
	state := h.states[id]
	now := time.Now().UTC()
	state.LastSeenAt = &now
	h.states[id] = state
}

// Status copies only management metadata; credentials and target addresses are
// deliberately absent. History is bounded by the configured node identities.
func (h *Hub) Status() HubStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := HubStatus{GeneratedAt: time.Now().UTC(), ActiveSessions: len(h.sessions), MaxSessions: h.max, Nodes: make([]NodeStatus, 0, len(h.nodes))}
	counts := map[string]int{}
	for _, session := range h.sessions {
		counts[session.targetID]++
		if session.sourceID != "" && session.sourceID != session.targetID {
			counts[session.sourceID]++
		}
	}
	for id := range h.nodes {
		state := h.states[id]
		state.ConnectedAt = copyTime(state.ConnectedAt)
		state.DisconnectedAt = copyTime(state.DisconnectedAt)
		state.LastSeenAt = copyTime(state.LastSeenAt)
		state.ExpiresAt = copyTime(state.ExpiresAt)
		state.Services = []string{}
		state.HookIDs = []string{}
		if control := h.controls[id]; control != nil {
			for service := range control.services {
				state.Services = append(state.Services, service)
			}
			for hook := range control.hooks {
				state.HookIDs = append(state.HookIDs, hook)
			}
		}
		state.ActiveSessions = counts[id]
		sort.Strings(state.Services)
		sort.Strings(state.HookIDs)
		result.Nodes = append(result.Nodes, state)
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	return result
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
