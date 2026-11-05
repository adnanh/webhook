package network

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"time"
)

const (
	nonceHeader = "X-Webhook-Network-Nonce"
	proofHeader = "X-Webhook-Network-Proof"
)

// connectionProof binds either peer's token possession to one connection request.
func connectionProof(tokenHash [32]byte, label string, nonce []byte, node, path, role, destination, session string) string {
	mac := hmac.New(sha256.New, tokenHash[:])
	_, _ = mac.Write([]byte(label))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(nonce)
	for _, field := range []string{node, path, role, destination, session} {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(field)))
		_, _ = mac.Write(size[:])
		_, _ = mac.Write([]byte(field))
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func requestProof(tokenHash [32]byte, label string, nonce []byte, r *http.Request) string {
	return connectionProof(tokenHash, label, nonce, r.Header.Get(nodeHeader), r.URL.Path,
		r.Header.Get(roleHeader), r.Header.Get(toHeader), r.Header.Get(sessionHeader))
}

// Timestamp plus 192 random bits: bounded replay protection without a second
// challenge round trip. Both peers must have reasonably synchronized clocks.
func newNonce() ([]byte, error) {
	nonce := make([]byte, 32)
	binary.BigEndian.PutUint64(nonce[:8], uint64(time.Now().Unix()))
	_, err := rand.Read(nonce[8:])
	return nonce, err
}

func (h *Hub) consumeNonce(id string, nonce []byte) bool {
	now := time.Now()
	stamp := int64(binary.BigEndian.Uint64(nonce[:8]))
	if stamp < now.Unix()-60 || stamp > now.Unix()+60 {
		return false
	}
	key := id + ":" + string(nonce)
	h.mu.Lock()
	defer h.mu.Unlock()
	if now.After(h.nextPrune) {
		for k, expiry := range h.replays {
			if !now.Before(expiry) {
				delete(h.replays, k)
			}
		}
		h.nextPrune = now.Add(time.Second)
	}
	if _, used := h.replays[key]; used || len(h.replays) >= 65536 {
		return false
	}
	h.replays[key] = time.Unix(stamp+61, 0)
	return true
}
