package httpx

import (
	"net/http"
	"strings"
)

// RobotsPath is where a crawler asks what it may fetch.
const RobotsPath = "/robots.txt"

// PreviewPath is the image a link to this site shows in a message or a post.
const PreviewPath = "/preview.png"

// robots answers what a crawler may fetch — which is everything, in both
// indexing modes.
//
// `robots.txt` answers "may this be fetched"; the `X-Robots-Tag` header
// answers "may this be listed". They are not alternatives: a crawler forbidden
// from fetching a page never sees the header refusing to list it, and can
// still list the address from somebody else's link, with no description,
// because it was never allowed to look. To refuse indexing, crawling has to be
// allowed (docs/requirements.md, section 7.1).
//
// What changes with the mode is one line: a sitemap is an invitation to index,
// so it is offered only when indexing is on, and a comment stands in its place
// so that whoever reads this file knows the line is missing on purpose.
func robots(indexable bool, sitemap string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var body strings.Builder
		body.WriteString("User-agent: *\n")
		body.WriteString("Allow: /\n")

		switch {
		case indexable && sitemap != "":
			body.WriteString("\nSitemap: " + sitemap + "\n")
		case indexable:
			// Indexing is on and there is nothing to list yet. The line is
			// absent rather than pointing at a sitemap that does not exist:
			// inviting a crawler to a 404 is worse than not inviting it.
			body.WriteString("\n# No sitemap yet: the first indexable pages arrive in phase 2.\n")
		default:
			body.WriteString("\n# No sitemap: this deployment is not indexable.\n")
			body.WriteString("# Crawling stays allowed so that crawlers can read the\n")
			body.WriteString("# X-Robots-Tag header, which is what refuses the listing.\n")
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(body.String()))
	}
}
