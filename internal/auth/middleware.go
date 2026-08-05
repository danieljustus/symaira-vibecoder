package auth

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

type TokenStore interface {
	Authenticate(token string) bool
}

func Middleware(next http.Handler, store TokenStore, bypassLoopback bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bypassLoopback && isLoopback(r) {
			next.ServeHTTP(w, r)
			return
		}

		if token := extractToken(r); token != "" && store.Authenticate(token) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
}

// OriginGuard rejects cross-site browser requests to the loopback board before
// the auth middleware runs. The board binds to 127.0.0.1 and the auth gate
// trusts loopback callers, so any page on the internet can otherwise fire a
// no-cors POST at http://127.0.0.1:4317/api/run (see internal/server
// handlers_run.go — /api/run reads no request body). The guard answers 403 when
// the browser itself labels the request cross-site (Sec-Fetch-Site:
// cross-site) or when the request carries an Origin whose hostname differs from
// the loopback Host it is addressed to. Requests without an Origin header —
// the SwiftUI client, curl, recipe/MCP callers — pass through unchanged, and
// non-loopback Hosts (lan/relay behind the token gate) are left to the auth
// middleware. allowOrigin is an escape hatch for embedding the board
// elsewhere: a request whose Origin matches it (or "*" for any origin) passes
// regardless of the loopback checks.
func OriginGuard(next http.Handler, allowOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !originAllowed(r, allowOrigin) {
			if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
				rejectForbidden(w)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && isLoopbackHost(r.Host) {
				if !hostsMatch(hostOnly(originHost(origin)), hostOnly(r.Host)) {
					rejectForbidden(w)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func rejectForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"forbidden"}`))
}

// originAllowed reports whether the request's Origin is explicitly permitted
// via the allow_origin escape hatch.
func originAllowed(r *http.Request, allowOrigin string) bool {
	if allowOrigin == "" {
		return false
	}
	if allowOrigin == "*" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	allowed := originHost(allowOrigin)
	return allowed != "" && hostsMatch(hostOnly(originHost(origin)), hostOnly(allowed))
}

// originHost extracts the host[:port] from an Origin header value.
func originHost(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return u.Host
}

// isLoopbackHost reports whether hostport names a loopback address: localhost
// or a loopback IP, with or without a port.
func isLoopbackHost(hostport string) bool {
	host := hostOnly(hostport)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// hostOnly strips the port (and IPv6 brackets) from a host[:port] value.
func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.Trim(h, "[]")
	}
	return strings.Trim(hostport, "[]")
}

func hostsMatch(a, b string) bool { return strings.EqualFold(a, b) }

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return r.URL.Query().Get("token")
}

func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
