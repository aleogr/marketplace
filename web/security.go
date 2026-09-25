package web

// Factor is a second factor as the security page lists it. Method is its kind
// as internal/identity names it (totp, webauthn, email); Added and LastUsed are
// already written in the page's language, and LastUsed is empty for a factor
// never used.
type Factor struct {
	ID       string
	Method   string
	Label    string
	Added    string
	LastUsed string
}

// Security is what the security page shows: the account's second factors,
// its recovery codes, what was just done, and a refusal, if there was one.
type Security struct {
	Factors        []Factor
	RecoveryIssued int
	RecoveryLeft   int
	// Done is the key of what was just done, for the status line, with its
	// arguments.
	Done     string
	DoneArgs []any
	Form     Form
}

// AppEnrolment is the page that adds an authenticator app: the QR code, as a
// data URI the policy's img-src allows, the key for typing it in instead,
// written in groups of four, and the label the visitor typed with its bound,
// which comes from internal/identity.
type AppEnrolment struct {
	QR       string
	Key      string
	Label    string
	LabelMax int
	Form     Form
}

// Recovery is the page that shows a fresh set of recovery codes, once.
// Added says the codes came with a method just added, which the page says
// first.
type Recovery struct {
	Codes []string
	Added bool
}
