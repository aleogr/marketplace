package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- RFC 6238's HMAC-SHA-1, the only algorithm every authenticator app accepts (spec, D6).
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// The TOTP parameters (RFC 6238; spec, D6): HMAC-SHA-1, six digits and
// thirty-second steps are the only ones every authenticator app accepts, and a
// 160-bit secret is what RFC 4226 recommends for SHA-1. A code is accepted one
// step either side of the current one, which is thirty seconds of clock drift.
const (
	totpDigits      = 6
	totpPeriod      = 30 // seconds
	totpSecretBytes = 20
	totpDrift       = 1
)

// NewTOTPSecret returns a secret nobody has seen.
func NewTOTPSecret() ([]byte, error) {
	secret := make([]byte, totpSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// totpStep is the step a moment falls in.
func totpStep(at time.Time) int64 { return at.Unix() / totpPeriod }

// hotp is RFC 4226: the HMAC of the counter, dynamically truncated to digits.
func hotp(secret []byte, counter int64, digits int) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(counter)) // #nosec G115 -- a step is never negative.
	mac := hmac.New(sha1.New, secret)
	mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	modulus := uint32(1)
	for range digits {
		modulus *= 10
	}
	return fmt.Sprintf("%0*d", digits, value%modulus)
}

// totpCode is the code of a step.
func totpCode(secret []byte, step int64) string { return hotp(secret, step, totpDigits) }

// matchTOTP returns the step a typed code belongs to, among the steps within
// the drift of now and after last, the last step accepted for this secret: a
// code is never accepted twice, nor one older than a code already accepted
// (spec, D6). Spaces are ignored, since apps show the code in two halves.
func matchTOTP(secret []byte, typed string, now time.Time, last int64) (int64, bool) {
	code := strings.ReplaceAll(typed, " ", "")
	if len(code) != totpDigits || strings.Trim(code, "0123456789") != "" {
		return 0, false
	}
	current := totpStep(now)
	for step := current - totpDrift; step <= current+totpDrift; step++ {
		if step <= last {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, step)), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// totpEncoding is how a secret is written for a person: RFC 4648 base32
// without padding, which is what apps ask for when the QR code cannot be read.
var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// TOTPKey is a secret as a person types it into an app.
func TOTPKey(secret []byte) string { return totpEncoding.EncodeToString(secret) }

// TOTPURI is what the enrolment QR code carries: the otpauth URI every app
// reads, labelled with the marketplace and the account so a person with
// several can tell them apart.
func TOTPURI(issuer, account string, secret []byte) string {
	escape := func(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }
	return "otpauth://totp/" + escape(issuer) + ":" + escape(account) +
		"?secret=" + TOTPKey(secret) + "&issuer=" + escape(issuer) +
		fmt.Sprintf("&algorithm=SHA1&digits=%d&period=%d", totpDigits, totpPeriod)
}
