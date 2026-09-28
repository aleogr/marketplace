package web

// MenuItem is one entry of the console's menu: the key of its label, the
// path it opens below the language, and whether it is the page being shown.
type MenuItem struct {
	Label   string
	Path    string
	Current bool
}
