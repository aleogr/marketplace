//go:build integration

package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

// softKey is an authenticator in software: what a security key does,
// written out, so the service's WebAuthn path is tested with no browser. It
// attests with "none", as the service asks, and signs with ECDSA P-256.
type softKey struct {
	t      *testing.T
	key    *ecdsa.PrivateKey
	id     []byte
	count  uint32
	origin string
}

func newSoftKey(t *testing.T, origin string) *softKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	return &softKey{t: t, key: key, id: id, origin: origin}
}

var b64 = base64.RawURLEncoding

// clientData is what the browser hands the authenticator to sign over.
func (k *softKey) clientData(kind, challenge string) []byte {
	raw, err := json.Marshal(map[string]any{"type": kind, "challenge": challenge, "origin": k.origin, "crossOrigin": false})
	if err != nil {
		k.t.Fatal(err)
	}
	return raw
}

// authData is the authenticator data: the relying party's hash, the flags
// (user present and verified), the counter, and what follows.
func (k *softKey) authData(flags byte, rest []byte) []byte {
	host, err := url.Parse(k.origin)
	if err != nil {
		k.t.Fatal(err)
	}
	rpHash := sha256.Sum256([]byte(host.Hostname()))
	data := append(rpHash[:], flags)
	data = binary.BigEndian.AppendUint32(data, k.count)
	return append(data, rest...)
}

// options reads the challenge of a creation or request's options.
func (k *softKey) challenge(options []byte) string {
	var parsed struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(options, &parsed); err != nil || parsed.PublicKey.Challenge == "" {
		k.t.Fatalf("the options carry no challenge: %s, %v", options, err)
	}
	return parsed.PublicKey.Challenge
}

// register answers navigator.credentials.create with the options given.
func (k *softKey) register(options []byte) []byte {
	cose, err := webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.EllipticKey), Algorithm: int64(webauthncose.AlgES256)},
		Curve:         1,
		XCoord:        k.key.PublicKey.X.FillBytes(make([]byte, 32)),
		YCoord:        k.key.PublicKey.Y.FillBytes(make([]byte, 32)),
	})
	if err != nil {
		k.t.Fatal(err)
	}
	attested := make([]byte, 16) // AAGUID: none
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(k.id)))
	attested = append(append(attested, k.id...), cose...)
	object, err := webauthncbor.Marshal(map[string]any{
		"fmt": "none", "attStmt": map[string]any{}, "authData": k.authData(0x45, attested),
	})
	if err != nil {
		k.t.Fatal(err)
	}
	return k.answer(map[string]any{
		"attestationObject": b64.EncodeToString(object),
		"clientDataJSON":    b64.EncodeToString(k.clientData("webauthn.create", k.challenge(options))),
	})
}

// assert answers navigator.credentials.get with the options given, moving
// its counter on as a real key does.
func (k *softKey) assert(options []byte) []byte {
	k.count++
	return k.sign(options)
}

// sign answers with the counter as it is, which a clone would.
func (k *softKey) sign(options []byte) []byte {
	data := k.authData(0x05, nil)
	client := k.clientData("webauthn.get", k.challenge(options))
	clientHash := sha256.Sum256(client)
	digest := sha256.Sum256(append(append([]byte{}, data...), clientHash[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, k.key, digest[:])
	if err != nil {
		k.t.Fatal(err)
	}
	return k.answer(map[string]any{
		"authenticatorData": b64.EncodeToString(data),
		"clientDataJSON":    b64.EncodeToString(client),
		"signature":         b64.EncodeToString(signature),
	})
}

func (k *softKey) answer(response map[string]any) []byte {
	raw, err := json.Marshal(map[string]any{
		"id": b64.EncodeToString(k.id), "rawId": b64.EncodeToString(k.id), "type": "public-key",
		"response": response, "clientExtensionResults": map[string]any{},
	})
	if err != nil {
		k.t.Fatal(err)
	}
	return raw
}
