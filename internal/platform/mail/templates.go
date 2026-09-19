package mail

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"path"
	"sort"
	"strings"
	texttemplate "text/template"

	"github.com/aleogr/marketplace/web"
)

// subjectPrefix is what the first line of a text part begins with. Keeping the
// subject in the same file as the body is what makes a translation one file to
// hand over and one file to review, instead of a body here and a subject in a
// catalogue somewhere else.
const subjectPrefix = "Subject: "

// Templates is every message the platform can send, in every language it
// speaks, parsed once at start-up.
//
// Parsed once and not per message: a template that does not compile is a
// deployment that refuses to start, which is minutes, rather than a message
// that fails to render months later, which is a person not receiving what they
// were promised.
type Templates struct {
	text map[string]*texttemplate.Template
	html map[string]*htmltemplate.Template
	// names is every template, sorted, for the parity check.
	names []string
	// languages is every language a template was found in.
	languages []string
}

// directory is where the templates live, in the embedded files and in the
// in-memory ones the tests load.
const directory = "mail"

// LoadTemplates reads the templates the binary carries.
func LoadTemplates() (*Templates, error) { return loadTemplates(web.Mail) }

func loadTemplates(files fs.FS) (*Templates, error) {
	entries, err := fs.ReadDir(files, directory)
	if err != nil {
		return nil, fmt.Errorf("cannot read the mail templates: %w", err)
	}

	loaded := &Templates{
		text: map[string]*texttemplate.Template{},
		html: map[string]*htmltemplate.Template{},
	}

	names, languages := map[string]bool{}, map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name, language, kind, err := parts(entry.Name())
		if err != nil {
			return nil, err
		}

		raw, err := fs.ReadFile(files, path.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", entry.Name(), err)
		}

		// missingkey=error, so that a template asking for a variable nobody
		// passed is an error a test sees rather than the words "no value" in
		// somebody's inbox.
		key := key(name, language)
		switch kind {
		case "txt":
			parsed, err := texttemplate.New(entry.Name()).
				Option("missingkey=error").Parse(string(raw))
			if err != nil {
				return nil, fmt.Errorf("cannot parse %s: %w", entry.Name(), err)
			}
			loaded.text[key] = parsed
		case "html":
			parsed, err := htmltemplate.New(entry.Name()).
				Option("missingkey=error").Parse(string(raw))
			if err != nil {
				return nil, fmt.Errorf("cannot parse %s: %w", entry.Name(), err)
			}
			loaded.html[key] = parsed
		}

		names[name], languages[language] = true, true
	}

	loaded.names, loaded.languages = sorted(names), sorted(languages)
	if len(loaded.names) == 0 {
		return nil, fmt.Errorf("no mail template was found in %s", directory)
	}

	// A template that exists in one language and not in another is a person
	// who receives nothing because of the language they chose
	// (docs/requirements.md, section 6).
	for _, name := range loaded.names {
		for _, language := range loaded.languages {
			for kind, exists := range map[string]bool{
				"text": loaded.text[key(name, language)] != nil,
				"HTML": loaded.html[key(name, language)] != nil,
			} {
				if !exists {
					return nil, fmt.Errorf("the template %q has no %s part in %s",
						name, kind, language)
				}
			}
		}
	}
	return loaded, nil
}

// parts splits a file name into the template it belongs to, the language it is
// written in and which part it is.
func parts(file string) (name, language, kind string, err error) {
	rest, kind, found := cutLast(file, ".")
	if !found || (kind != "txt" && kind != "html") {
		return "", "", "", fmt.Errorf("%q is neither a .txt nor a .html part", file)
	}

	name, language, found = strings.Cut(rest, ".")
	if !found || name == "" || language == "" {
		return "", "", "", fmt.Errorf(
			"%q is not named <template>.<language>.<txt|html>", file)
	}
	return name, language, kind, nil
}

func cutLast(s, sep string) (before, after string, found bool) {
	index := strings.LastIndex(s, sep)
	if index < 0 {
		return s, "", false
	}
	return s[:index], s[index+len(sep):], true
}

func key(name, language string) string { return name + "." + language }

func sorted(set map[string]bool) []string {
	list := make([]string, 0, len(set))
	for item := range set {
		list = append(list, item)
	}
	sort.Strings(list)
	return list
}

// Names returns every template, sorted.
func (t *Templates) Names() []string { return append([]string(nil), t.names...) }

// Languages returns every language the templates were found in, sorted.
func (t *Templates) Languages() []string { return append([]string(nil), t.languages...) }

// Render fills a message's template with its variables.
//
// A language the templates do not have falls back to the official one, which
// is the same rule the catalogues follow: a message in English reaches its
// reader, and a message that failed to render does not
// (docs/requirements.md, section 6).
func (t *Templates) Render(message Message, fallback string) (Rendered, error) {
	language := message.Language
	if t.text[key(message.Template, language)] == nil {
		language = fallback
	}

	text, html := t.text[key(message.Template, language)], t.html[key(message.Template, language)]
	if text == nil || html == nil {
		return Rendered{}, fmt.Errorf("%w: %s in %s", ErrNoTemplate, message.Template, message.Language)
	}

	// What every template may use, whatever it is about: who it is from, and
	// which language it came out in. Anything else is the caller's to pass.
	data := map[string]string{
		"From":     message.From,
		"Language": language,
	}
	for name, value := range message.Variables {
		data[name] = value
	}

	var body bytes.Buffer
	if err := text.Execute(&body, data); err != nil {
		return Rendered{}, fmt.Errorf("cannot render the text part of %s in %s: %w",
			message.Template, language, err)
	}

	subject, plain, err := split(body.String())
	if err != nil {
		return Rendered{}, fmt.Errorf("%s in %s: %w", message.Template, language, err)
	}

	var markup bytes.Buffer
	if err := html.Execute(&markup, data); err != nil {
		return Rendered{}, fmt.Errorf("cannot render the HTML part of %s in %s: %w",
			message.Template, language, err)
	}

	rendered := Rendered{Message: message, Subject: subject, Text: plain, HTML: markup.String()}
	rendered.Language = language
	return rendered, nil
}

// split separates the subject line from the body of a text part.
func split(rendered string) (subject, body string, err error) {
	line, rest, found := strings.Cut(rendered, "\n")
	if !found {
		return "", "", fmt.Errorf("the text part has no body")
	}

	subject, found = strings.CutPrefix(strings.TrimSpace(line), subjectPrefix)
	if !found || subject == "" {
		return "", "", fmt.Errorf("the first line is not %q", strings.TrimSpace(subjectPrefix))
	}
	return subject, strings.TrimLeft(rest, "\n"), nil
}
