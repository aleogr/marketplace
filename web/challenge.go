package web

// Challenge is the page that asks for a second factor: the second step of a
// sign-in, or a step-up before a sensitive action (Action names it; empty at
// sign-in). Methods are what the account may answer with, strongest first,
// and Method the one shown; Recovery says a recovery code may answer. Base is
// this page's own address with its query so far, ending in ? or &, which the
// links to the other methods complete. Method is empty when nothing is left
// to answer with.
type Challenge struct {
	Action   string
	Methods  []string
	Method   string
	Recovery bool
	// CodesLocked says the account's codes are refused after too many
	// failed second factors in a row, until its password is changed.
	CodesLocked bool
	Base        string
	// Next is where a step-up returns to, as a path below the language.
	Next string
	// Sent says an e-mail code was just sent.
	Sent bool
	// KeyOptions are what the browser's navigator.credentials.get takes, as
	// JSON, when the method shown is a key.
	KeyOptions string
	Form       Form
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
