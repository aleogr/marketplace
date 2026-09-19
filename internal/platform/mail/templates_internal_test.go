package mail

import (
	"strings"
	"testing"
	"testing/fstest"
)

// complete is a pair of templates in two languages, as the binary carries
// them.
func complete() fstest.MapFS {
	return fstest.MapFS{
		"mail/hello.en-US.txt":  {Data: []byte("Subject: Hello\n\nHello, {{.From}}.\n")},
		"mail/hello.en-US.html": {Data: []byte("<p>Hello, {{.From}}.</p>\n")},
		"mail/hello.pt-BR.txt":  {Data: []byte("Subject: Olá\n\nOlá, {{.From}}.\n")},
		"mail/hello.pt-BR.html": {Data: []byte("<p>Olá, {{.From}}.</p>\n")},
	}
}

func TestACompleteSetLoads(t *testing.T) {
	loaded, err := loadTemplates(complete())
	if err != nil {
		t.Fatalf("loadTemplates() = %v", err)
	}
	if got := loaded.Languages(); len(got) != 2 {
		t.Errorf("languages = %v, want two", got)
	}
}

// The check that makes the set complete. Removing one part must be a process
// that refuses to start, because the alternative is a reader who receives
// nothing and nobody who knows.
func TestAMissingPartRefusesToLoad(t *testing.T) {
	for _, missing := range []string{
		"mail/hello.pt-BR.html",
		"mail/hello.pt-BR.txt",
		"mail/hello.en-US.html",
	} {
		t.Run(missing, func(t *testing.T) {
			files := complete()
			delete(files, missing)

			_, err := loadTemplates(files)
			if err == nil {
				t.Fatalf("loadTemplates() accepted a set with %s missing", missing)
			}
			if !strings.Contains(err.Error(), "hello") {
				t.Errorf("the error does not name the template: %v", err)
			}
		})
	}
}

func TestAFileThatIsNotNamedForATemplateRefusesToLoad(t *testing.T) {
	for _, name := range []string{"mail/hello.txt", "mail/hello.en-US.md"} {
		files := complete()
		files[name] = &fstest.MapFile{Data: []byte("Subject: x\n\nx\n")}

		if _, err := loadTemplates(files); err == nil {
			t.Errorf("loadTemplates() accepted %q", name)
		}
	}
}

func TestATextPartWithNoSubjectLineIsAnError(t *testing.T) {
	files := complete()
	files["mail/hello.en-US.txt"] = &fstest.MapFile{Data: []byte("Hello, {{.From}}.\n")}

	loaded, err := loadTemplates(files)
	if err != nil {
		t.Fatalf("loadTemplates() = %v", err)
	}
	if _, err := loaded.Render(Message{Template: "hello", Language: "en-US"}, "en-US"); err == nil {
		t.Error("a template with no subject line rendered; the message would arrive with none")
	}
}

// A template asking for a variable nobody passed must be an error here rather
// than the words "no value" in somebody's inbox.
func TestAVariableNobodyPassedIsAnError(t *testing.T) {
	files := complete()
	files["mail/hello.en-US.txt"] = &fstest.MapFile{
		Data: []byte("Subject: Hello\n\nHello, {{.Nobody}}.\n"),
	}

	loaded, err := loadTemplates(files)
	if err != nil {
		t.Fatalf("loadTemplates() = %v", err)
	}
	if _, err := loaded.Render(Message{Template: "hello", Language: "en-US"}, "en-US"); err == nil {
		t.Error("a template referring to a variable nobody passed rendered anyway")
	}
}

// The HTML part escapes what it is given: a name carrying markup is a name,
// not markup.
func TestTheHTMLPartEscapesWhatItIsGiven(t *testing.T) {
	loaded, err := loadTemplates(complete())
	if err != nil {
		t.Fatalf("loadTemplates() = %v", err)
	}

	rendered, err := loaded.Render(Message{
		Template: "hello", Language: "en-US", From: "<script>alert(1)</script>",
	}, "en-US")
	if err != nil {
		t.Fatalf("Render() = %v", err)
	}
	if strings.Contains(rendered.HTML, "<script>") {
		t.Errorf("the HTML part carries markup it was given as text: %q", rendered.HTML)
	}
}
