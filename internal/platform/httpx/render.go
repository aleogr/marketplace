package httpx

import (
	"net/http"

	"github.com/a-h/templ"
)

// render writes a component as the response.
//
// A template that fails halfway has already written part of a page, so there is
// no status left to change and nothing useful to say to the visitor; what there
// is to do is not pretend it succeeded, which is why the error is dropped here
// and the failure shows up as a truncated response and in the access log rather
// than as a second set of headers Go would refuse to write anyway.
func render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	renderStatus(w, r, http.StatusOK, component)
}

// renderStatus is render with a status other than 200: a form shown again
// with what was wrong with it. The type is set before the status is written,
// because a header set after WriteHeader is silently dropped.
func renderStatus(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = component.Render(r.Context(), w)
}
