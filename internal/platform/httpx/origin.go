package httpx

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// Origin is where a request came from: the address of the client and what it
// said it was running.
//
// It is read once, at the edge of the pipeline, and carried in the context —
// rate limiting keys on the address, and auditing records both (F12). Reading
// it once means every consumer agrees on the answer.
type Origin struct {
	IP        string
	UserAgent string
}

type originKey struct{}

// WithOrigin returns a context carrying o.
func WithOrigin(ctx context.Context, o Origin) context.Context {
	return context.WithValue(ctx, originKey{}, o)
}

// OriginFrom returns the origin of the request, if the middleware ran.
func OriginFrom(ctx context.Context) (Origin, bool) {
	o, ok := ctx.Value(originKey{}).(Origin)
	return o, ok
}

// Origins reads the origin of each request and puts it in the context.
func Origins(hops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := Origin{
				IP:        ClientIP(r, hops),
				UserAgent: r.UserAgent(),
			}
			next.ServeHTTP(w, r.WithContext(WithOrigin(r.Context(), origin)))
		})
	}
}

// ClientIP returns the address of the client, counting hops entries back from
// the right of `X-Forwarded-For`.
//
// Counting from the right is the only safe direction. Google's front end
// appends to that header rather than replacing it, so a client that sends its
// own `X-Forwarded-For: 1.2.3.4` is served as
// `1.2.3.4, <its real address>, <the front end>`: everything to the left of
// what the infrastructure added is written by whoever is being limited. Taking
// the leftmost entry — the usual reading, and the one most libraries default
// to — would let a client choose its own rate-limit bucket by choosing a new
// address on every request.
//
// How many entries the platform adds is not in Cloud Run's container contract
// and was not found in its documentation, which is why it is configuration
// (`TRUSTED_PROXY_HOPS`) and not a constant, and why the value in use is
// checked against the deployment rather than assumed
// (docs/infrastructure.md).
//
// An address that is not there, or not an address, falls back to the peer of
// the connection. That is the front end rather than the client, so a
// misconfigured hop count limits everyone together instead of limiting nobody:
// of the two ways to be wrong, this is the one that fails closed.
func ClientIP(r *http.Request, hops int) string {
	forwarded := r.Header.Values("X-Forwarded-For")
	var chain []string
	for _, value := range forwarded {
		for _, entry := range strings.Split(value, ",") {
			if entry = strings.TrimSpace(entry); entry != "" {
				chain = append(chain, entry)
			}
		}
	}

	if index := len(chain) - hops; hops > 0 && index >= 0 && index < len(chain) {
		if ip := parseIP(chain[index]); ip != "" {
			return ip
		}
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// parseIP accepts an address with or without a port, and returns "" for
// anything that is not an address at all.
func parseIP(entry string) string {
	if ip := net.ParseIP(entry); ip != nil {
		return ip.String()
	}
	if host, _, err := net.SplitHostPort(entry); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	return ""
}
