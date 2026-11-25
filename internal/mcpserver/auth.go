package mcpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

// HashAccessKey returns the persisted form of an MCP access key: the hex
// SHA-256 of the trimmed key. Bearer tokens only need verification, so a
// hash avoids storing the secret itself and needs no vault unlock.
func HashAccessKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

// authorized reports whether the Authorization header carries the bearer
// token matching hash. An empty hash disables authentication (loopback
// only — the caller is expected to warn about that in the UI).
func authorized(header, hash string) bool {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return true
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(HashAccessKey(strings.TrimPrefix(header, prefix))),
		[]byte(hash),
	) == 1
}

// authMiddleware rejects requests without a valid bearer token before they
// reach the MCP streamable-HTTP handler.
func authMiddleware(next http.Handler, hash string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r.Header.Get("Authorization"), hash) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="vela-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
