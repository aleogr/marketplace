package httpx

import (
	"net/http"

	"github.com/aleogr/marketplace/internal/tenancy"
)

// The console lives on a host of its own (F15 spec, D1): the staff's sign-in
// and the console's pages, and nothing of a marketplace.

// onConsole reports whether a request reached the console's host.
func onConsole(r *http.Request) bool {
	resolution, ok := tenancy.FromContext(r.Context())
	return ok && resolution.Kind == tenancy.ConsoleHost
}

// consoleRoutes are what the console's host serves. The machinery every host
// has comes first; robots.txt says what it says on a deployment that is not
// indexable, whatever this one's setting, because the console never is.
func (s Site) consoleRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+HealthPath, health(s.database))
	mux.HandleFunc("GET "+RobotsPath, robots(false, ""))
	mux.HandleFunc("GET "+ScriptPath, script)
	mux.HandleFunc("POST "+LanguagePath, s.switchLanguage)
	return mux
}
