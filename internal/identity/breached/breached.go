// Package breached answers whether a password has appeared in a public breach
// (spec, D4). It is a port with two adapters: Pwned Passwords, and a fake.
package breached

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1" // #nosec G505 -- SHA-1 is the range API's protocol, not a password hash
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Checker is the port.
type Checker interface {
	Breached(ctx context.Context, password string) (bool, error)
}

// RangeAPI is the address of the Pwned Passwords range API.
const RangeAPI = "https://api.pwnedpasswords.com"

// maxRangeBody bounds how much of a range response is read. The real API's
// padded bucket is well under this; anything larger is treated as an error
// so the caller's fail-open path applies, rather than silently scanning a
// truncated body and calling the password clean.
const maxRangeBody = 1 << 20 // 1 MiB

// Pwned asks the range API with the first five hexadecimal characters of the
// password's SHA-1 and compares the rest locally (k-anonymity). Padding hides
// even the number of matches from anyone watching the response size.
type Pwned struct {
	client *http.Client
	base   string
}

// NewPwned returns the adapter. client carries the timeout.
func NewPwned(client *http.Client, base string) *Pwned { return &Pwned{client: client, base: base} }

// Breached implements Checker.
func (p *Pwned) Breached(ctx context.Context, password string) (bool, error) {
	sum := sha1.Sum([]byte(password)) // #nosec G401 -- see the import
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := full[:5], full[5:]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base+"/range/"+prefix, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Add-Padding", "true")
	req.Header.Set("User-Agent", "aleogr-marketplace")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("breached: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("breached: the range API answered %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRangeBody+1))
	if err != nil {
		return false, fmt.Errorf("breached: %w", err)
	}
	if len(body) > maxRangeBody {
		return false, fmt.Errorf("breached: the range API response exceeded %d bytes", maxRangeBody)
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		found, count, ok := strings.Cut(line, ":")
		if ok && found == suffix && count != "0" {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// Common are the passwords the fake calls breached, so the suite can prove a
// refusal without reaching the internet.
var Common = map[string]bool{"password1234": true, "123456789012": true, "senhasenha123": true}

// Fake is the adapter tests and a local run use. Known is what it calls
// breached; Err, when set, is what it answers instead, which is how a test
// makes the service unreachable.
type Fake struct {
	Known map[string]bool
	Err   error
}

// Breached implements Checker.
func (f Fake) Breached(_ context.Context, password string) (bool, error) {
	return f.Known[password], f.Err
}
