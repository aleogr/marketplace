// Package geoip answers which country an address is in.
//
// It is a port with one fake until F18 brings the real database
// (docs/design.md, section 2.2). Detection by country is the last thing
// language resolution consults and the first thing a visitor overrides, so a
// deployment that knows no countries is a working deployment: everyone gets
// the official language until they say otherwise (docs/requirements.md,
// section 6).
package geoip

// Locator answers which country an address is in, as an ISO 3166-1 alpha-2
// code, and whether it knows at all.
type Locator interface {
	Country(ip string) (string, bool)
}

// Nowhere knows no addresses. It is the adapter a deployment without the
// database uses, and it says so by answering no rather than by guessing.
type Nowhere struct{}

// Country always answers that it does not know.
func (Nowhere) Country(string) (string, bool) { return "", false }

// Fixed answers one country for every address. Tests use it to exercise the
// path a real database will take.
type Fixed string

// Country answers the country this locator was built with.
func (f Fixed) Country(string) (string, bool) { return string(f), f != "" }
