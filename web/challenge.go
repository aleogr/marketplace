package web

// Challenge is the page that asks for a second factor: the second step of a
// sign-in, or a step-up before a sensitive action (Action names it; empty at
// sign-in). Methods are what the account may answer with, strongest first,
// and Method the one shown; Recovery says a recovery code may answer. Base is
// this page's own address with its query so far, ending in ? or &, which the
// links to the other methods complete.
type Challenge struct {
	Action   string
	Methods  []string
	Method   string
	Recovery bool
	Base     string
	// Next is where a step-up returns to, as a path below the language.
	Next string
	Form Form
}

// Others are the methods the page offers besides the one shown.
func (c Challenge) Others() []string {
	var others []string
	for _, method := range c.Methods {
		if method != c.Method {
			others = append(others, method)
		}
	}
	return others
}
