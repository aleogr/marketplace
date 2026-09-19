package httpx_test

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
)

// fetch asks the site for a path in one indexing mode.
func fetch(t *testing.T, indexable bool, path string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	site(t, nil, indexable).ServeHTTP(recorder,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return recorder
}

// TestCrawlingIsAllowedInBothModes is the rule that is easiest to get backwards.
//
// robots.txt answers "may this be fetched"; the header answers "may this be
// listed". Blocking the fetch hides the refusal to list, and the address can
// still be listed from somebody else's link, with no description, because the
// crawler was never allowed to look (docs/requirements.md, section 7.1).
func TestCrawlingIsAllowedInBothModes(t *testing.T) {
	for _, indexable := range []bool{false, true} {
		body := fetch(t, indexable, httpx.RobotsPath).Body.String()

		if !strings.Contains(body, "Allow: /") {
			t.Errorf("indexable=%v: robots.txt does not allow crawling:\n%s", indexable, body)
		}
		if strings.Contains(body, "Disallow: /") {
			t.Errorf("indexable=%v: robots.txt forbids crawling, which hides the refusal to index:\n%s",
				indexable, body)
		}
	}
}

func TestTheSitemapIsOfferedOnlyWhenIndexingIsOn(t *testing.T) {
	off := fetch(t, false, httpx.RobotsPath).Body.String()

	if strings.Contains(off, "Sitemap:") {
		t.Errorf("a deployment that refuses indexing still invites it:\n%s", off)
	}
	// The line is missing on purpose, and says so where somebody reading the
	// file will see it.
	if !strings.Contains(off, "# No sitemap") {
		t.Errorf("robots.txt drops the sitemap with no explanation:\n%s", off)
	}
}

// TestThePreviewImageIsServedInBothModes: a link pasted into a message should
// look like something whether or not search engines may list the page. That is
// why crawling stays allowed.
func TestThePreviewImageIsServedInBothModes(t *testing.T) {
	for _, indexable := range []bool{false, true} {
		response := fetch(t, indexable, httpx.PreviewPath)

		if response.Code != http.StatusOK {
			t.Fatalf("indexable=%v: the preview answered %d", indexable, response.Code)
		}
		if got := response.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("indexable=%v: Content-Type = %q", indexable, got)
		}

		image, err := png.Decode(bytes.NewReader(response.Body.Bytes()))
		if err != nil {
			t.Fatalf("indexable=%v: the preview is not a readable PNG: %v", indexable, err)
		}
		// The proportions every social network expects; an image of another
		// shape is cropped by whoever shows it.
		if bounds := image.Bounds(); bounds.Dx() != 1200 || bounds.Dy() != 630 {
			t.Errorf("the preview is %dx%d, want 1200x630", bounds.Dx(), bounds.Dy())
		}
	}
}

func TestThePageDescribesItselfOnceToEverybody(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := httpx.Secure(false)(site(t, nil, false))
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "https://marketplace1.example/", nil))

	body := recorder.Body.String()

	description := between(body, `<meta name="description" content="`, `"`)
	if description == "" {
		t.Fatalf("the page carries no description:\n%s", body)
	}

	for _, tag := range []string{
		`<meta property="og:description" content="`,
		`<meta name="twitter:description" content="`,
	} {
		if got := between(body, tag, `"`); got != description {
			t.Errorf("%s says %q, and the description tag says %q; they come from one source",
				tag, got, description)
		}
	}

	if image := between(body, `<meta property="og:image" content="`, `"`); !strings.HasSuffix(image, httpx.PreviewPath) {
		t.Errorf("og:image = %q, want the preview image", image)
	}
	if url := between(body, `<meta property="og:url" content="`, `"`); !strings.HasPrefix(url, "https://") {
		t.Errorf("og:url = %q, want an absolute address", url)
	}
}

// TestNoPageClaimsToBeIndexable covers the contradiction the requirements name:
// a page stating `<meta name="robots" content="index, follow">` would
// contradict the header, and conflicting directives resolve to the most
// restrictive one — so the claim is never true and always confusing
// (docs/requirements.md, section 7.1).
func TestNoPageClaimsToBeIndexable(t *testing.T) {
	for _, path := range []string{"/", "/en-US/"} {
		body := fetch(t, true, path).Body.String()

		if strings.Contains(strings.ToLower(body), `name="robots"`) {
			t.Errorf("%s states a robots meta tag, which contradicts the header:\n%s", path, body)
		}
	}
}

// between returns what lies between two markers, or "".
func between(text, start, end string) string {
	_, after, found := strings.Cut(text, start)
	if !found {
		return ""
	}
	value, _, found := strings.Cut(after, end)
	if !found {
		return ""
	}
	return value
}
