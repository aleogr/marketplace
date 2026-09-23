package breached

import (
	"context"
	"crypto/sha1" // #nosec G505 -- the range API's protocol is SHA-1
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPwnedSendsOnlyThePrefixAndFindsTheSuffix(t *testing.T) {
	sum := sha1.Sum([]byte("password1234")) // #nosec G401 -- see above
	full := strings.ToUpper(hex.EncodeToString(sum[:]))

	var asked, padding string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked, padding = r.URL.Path, r.Header.Get("Add-Padding")
		_, _ = w.Write([]byte("0000000000000000000000000000000000A:0\r\n" + full[5:] + ":12\r\n"))
	}))
	defer server.Close()

	found, err := NewPwned(server.Client(), server.URL).Breached(context.Background(), "password1234")
	if err != nil || !found {
		t.Fatalf("Breached = %v, %v; want true, nil", found, err)
	}
	if asked != "/range/"+full[:5] {
		t.Fatalf("asked %q, want only the five-character prefix", asked)
	}
	if padding != "true" {
		t.Fatal("the request did not ask for padding")
	}
}

func TestAPaddedZeroCountIsNotABreach(t *testing.T) {
	sum := sha1.Sum([]byte("an unusual passphrase")) // #nosec G401 -- see above
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(full[5:] + ":0\r\n"))
	}))
	defer server.Close()
	if found, _ := NewPwned(server.Client(), server.URL).Breached(context.Background(), "an unusual passphrase"); found {
		t.Fatal("a padding line with count 0 was taken as a breach")
	}
}

func TestTheFakeAnswersFromItsList(t *testing.T) {
	fake := Fake{Known: Common}
	if found, err := fake.Breached(context.Background(), "password1234"); !found || err != nil {
		t.Fatalf("Breached(listed) = %v, %v; want true, nil", found, err)
	}
	if found, _ := fake.Breached(context.Background(), "a passphrase nobody has used"); found {
		t.Fatal("the fake called an unlisted password breached")
	}
}

func TestAnOversizedBodyIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, maxRangeBody+1))
	}))
	defer server.Close()
	if found, err := NewPwned(server.Client(), server.URL).Breached(context.Background(), "whatever"); err == nil {
		t.Fatalf("Breached(oversized body) = %v, nil; want an error", found)
	}
}

func TestAnUnreachableServiceIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if _, err := NewPwned(server.Client(), server.URL).Breached(context.Background(), "whatever"); err == nil {
		t.Fatal("a 503 was not reported as an error")
	}
}
