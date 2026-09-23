package httpx

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/seo"
	"github.com/aleogr/marketplace/internal/tenancy"
	"github.com/aleogr/marketplace/web"
)

// LanguagePath is where the manual language switch posts.
//
// It is outside the language prefixes, because it is what a visitor uses to
// change which prefix they are under: an address that carried a language would
// have to be reached in the language being left behind.
const LanguagePath = "/language"

// Site serves the pages a person reads.
type Site struct {
	database  Database
	catalogue *i18n.Catalogue
	preview   *seo.Preview
	// mail answers the e-mail provider's webhooks. It is nil where no shared
	// token is configured: an endpoint standing in front of nothing is an open
	// door (internal/platform/config.Mail).
	mail *MailWebhook
	// tasks answers the callbacks of Cloud Tasks and Cloud Scheduler. It is
	// nil in a process with no database, which has no work to hand over.
	tasks *Tasks
	// indexable is the deployment's own setting. It changes one line of
	// robots.txt and nothing else: the refusal itself is a header on every
	// response (docs/requirements.md, section 7.1).
	indexable bool
	// identity serves sign-up, sign-in and the account's pages. It is nil in
	// a process with no database, which has no accounts to serve.
	identity *IdentityRoutes
}

// NewSite returns the site's routes, ready to be mounted behind the pipeline.
func NewSite(database Database, catalogue *i18n.Catalogue, preview *seo.Preview, indexable bool) Site {
	return Site{database: database, catalogue: catalogue, preview: preview, indexable: indexable}
}

// WithTasks returns the site answering the internal callbacks too.
func (s Site) WithTasks(tasks Tasks) Site {
	s.tasks = &tasks
	return s
}

// WithMail returns the site answering one e-mail provider's webhooks too.
func (s Site) WithMail(webhook MailWebhook) Site {
	s.mail = &webhook
	return s
}

// Handler returns the routes served by the process.
func (s Site) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+HealthPath, health(s.database))
	mux.HandleFunc("GET "+RobotsPath, robots(s.indexable, s.sitemap()))
	mux.HandleFunc("GET "+PreviewPath, s.previewImage)
	mux.HandleFunc("POST "+LanguagePath, s.switchLanguage)
	if s.tasks != nil {
		mux.HandleFunc("POST "+TasksPath, s.tasks.Handle)
	}
	if s.mail != nil {
		mux.HandleFunc("POST "+MailWebhookPrefix+"{provider}", s.mail.Handle)
	}
	s.identityRoutes(mux)
	mux.HandleFunc("GET /{$}", s.home)
	return mux
}

// sitemap is where the sitemap will be once there are pages to list. It is
// named here, rather than where it is served, because robots.txt is what
// offers it and phase 2 is what writes it (docs/roadmap.md, F9).
func (s Site) sitemap() string { return "" }

// previewImage draws the image a link to this marketplace shows.
//
// It is served in both indexing modes: a link somebody pastes into a message
// should look like something whether or not search engines may list the page
// (docs/requirements.md, section 7.1).
func (s Site) previewImage(w http.ResponseWriter, r *http.Request) {
	name := s.catalogue.Printer(i18n.Default).Sprintf("page.platform.title")
	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		name = resolution.Marketplace.Name
	}

	drawn, err := s.preview.PNG(name)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(drawn)
}

// home is a marketplace's own address, in the language resolved for it.
func (s Site) home(w http.ResponseWriter, r *http.Request) {
	page := s.page(r, "/")

	if page.Marketplace == "" {
		// The platform's own host, which belongs to no marketplace.
		render(w, r, web.Platform(page))
		return
	}
	render(w, r, web.Home(page))
}

// switchLanguage remembers what a visitor chose and sends them back where they
// were, in that language.
func (s Site) switchLanguage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	languages, fallback := Speaks(s.catalogue)(r)
	chosen := r.PostFormValue("language")
	if !spoken(languages, chosen) {
		chosen = fallback
	}

	i18n.Remember(w, r, chosen)

	// Back to the page they were reading, in the language they chose. The path
	// comes from the form and is therefore a visitor's to write, so only its
	// path is used and never a host: a redirect that took a whole address from
	// a form is an open redirect, and an open redirect is a phishing link with
	// this deployment's name on it.
	target := url.URL{Path: "/" + chosen + safePath(r.PostFormValue("path"))}
	// #nosec G710 -- safePath keeps a path and discards a host; see above.
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

// safePath keeps what a visitor may decide — where on this site to return to —
// and discards what they may not: which site.
func safePath(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/"
	}
	if strings.ContainsAny(path, "\\\r\n") {
		return "/"
	}
	return path
}

// page prepares everything a template may need for this request.
func (s Site) page(r *http.Request, path string) web.Page {
	tag := i18n.FromContext(r.Context())
	languages, _ := Speaks(s.catalogue)(r)

	page := web.NewPage(tag, s.catalogue.Printer(tag), s.names(languages))
	page.Nonce = Nonce(r.Context())
	page.CSRFToken = CSRFToken(r.Context())
	page.Path = path
	page.Default = "/" + i18n.Default + path

	// Absolute, because these are read by other people's servers when somebody
	// pastes a link: a path has nothing to resolve against there.
	base := origin(r)
	page.URL = base + "/" + tag + path
	page.Image = base + PreviewPath

	for _, language := range languages {
		if language != tag {
			page.Alternates[language] = "/" + language + path
		}
	}

	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		page.Marketplace = resolution.Marketplace.Name
	}

	// One source for every description the page carries, cut once
	// (docs/requirements.md, section 7.2).
	if page.Marketplace == "" {
		page.Description = seo.Description(page.T("page.platform.description"))
	} else {
		page.Description = seo.Description(page.T("page.home.description", page.Marketplace))
	}
	return page
}

// origin is the address this request reached the site at, as another server
// would have to write it.
func origin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// names is each language's name in its own language, which is how a person
// looks for it.
func (s Site) names(languages []string) map[string]string {
	names := map[string]string{}
	for _, language := range languages {
		names[language] = s.catalogue.Printer(language).Sprintf("language.name")
	}
	return names
}

// Speaks reports which languages a request may be served in: the marketplace's
// own, or every language the platform has when the host belongs to no
// marketplace.
func Speaks(catalogue *i18n.Catalogue) i18n.Enabled {
	return func(r *http.Request) ([]string, string) {
		resolution, ok := tenancy.FromContext(r.Context())
		if !ok || resolution.Marketplace == nil {
			return catalogue.Languages(), i18n.Default
		}
		return resolution.Marketplace.Languages, resolution.Marketplace.DefaultLanguage
	}
}

func spoken(languages []string, tag string) bool {
	for _, language := range languages {
		if language == tag {
			return true
		}
	}
	return false
}
