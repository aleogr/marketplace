package httpx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
)

// nonceBytes is how much randomness each Content Security Policy nonce
// carries. The specification asks for at least 128 bits; this is more, and the
// cost is a few bytes per response.
const nonceBytes = 16

// hsts is a year, which is what a browser is asked to remember. It carries no
// `preload` directive: preloading is a submission to a list shipped inside
// browsers, and undoing it takes months, so it is a decision for the first
// production launch rather than a default of the lab.
const hsts = "max-age=31536000; includeSubDomains"

type nonceKey struct{}

// Nonce returns the Content Security Policy nonce of the current request, for
// the templates that mark their own script and style tags with it.
func Nonce(ctx context.Context) string {
	nonce, _ := ctx.Value(nonceKey{}).(string)
	return nonce
}

// Secure mounts the headers the binary defends itself with.
//
// There is no load balancer and no web application firewall in front of this
// process (docs/requirements.md, section 24), so every header a reverse proxy
// would normally add is added here, once, for every response.
//
// indexable is the deployment's own setting: the lab must never appear in a
// search engine, and saying so in a header is what makes that true for a
// crawler that was pointed at it directly (docs/requirements.md, section 7.1).
func Secure(indexable bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce := newNonce()
			header := w.Header()

			// The policy is written for what the interface is built from:
			// templ renders the markup, HTMX and Alpine.js run in the page
			// (docs/requirements.md, section 24). Both are served from this
			// origin and marked with the nonce, so no host allow-list and no
			// 'unsafe-inline' are needed. Alpine.js needs
			// 'unsafe-eval'-like evaluation for its expressions, which
			// 'strict-dynamic' does not grant: its build without the
			// evaluator is what this policy assumes, and the delivery that
			// mounts Alpine.js proves it in a browser (docs/roadmap.md, F9).
			header.Set("Content-Security-Policy", strings.Join([]string{
				"default-src 'self'",
				"script-src 'self' 'nonce-" + nonce + "'",
				"style-src 'self' 'nonce-" + nonce + "'",
				"img-src 'self' data: https:",
				"font-src 'self'",
				"connect-src 'self'",
				"form-action 'self'",
				"base-uri 'none'",
				"object-src 'none'",
				// Nobody frames this: clickjacking needs a frame, and the
				// modern spelling of X-Frame-Options is this directive.
				"frame-ancestors 'none'",
			}, "; "))

			header.Set("X-Content-Type-Options", "nosniff")
			// Referrers leak. Another origin learns which site a visitor came
			// from and nothing more; this one still sees the whole path.
			header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Powerful features nothing here uses. Payment is denied too: the
			// gateway's checkout runs on its own origin, not in this document.
			header.Set("Permissions-Policy", strings.Join([]string{
				"accelerometer=()", "camera=()", "geolocation=()",
				"gyroscope=()", "magnetometer=()", "microphone=()",
				"payment=()", "usb=()",
			}, ", "))

			// Only over HTTPS. A browser ignores it on a plain connection, and
			// sending it from a local process would still be a claim this
			// process cannot make about the address it is reached at.
			if isHTTPS(r) {
				header.Set("Strict-Transport-Security", hsts)
			}

			if !indexable {
				header.Set("X-Robots-Tag", "noindex, nofollow")
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), nonceKey{}, nonce)))
		})
	}
}

// isHTTPS reports whether the request reached the front end over TLS. On Cloud
// Run the connection to the container is plain, so the answer is the header the
// front end sets and not r.TLS.
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func newNonce() string {
	buf := make([]byte, nonceBytes)
	// crypto/rand.Read is documented never to fail, and its error return is
	// kept only for compatibility.
	_, _ = rand.Read(buf)
	return base64.RawStdEncoding.EncodeToString(buf)
}
