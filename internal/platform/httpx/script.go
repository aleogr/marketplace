package httpx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/aleogr/marketplace/web"
)

// ScriptPath is where the site's one script is served: the WebAuthn ceremony
// of the security keys (F14). It is outside the language prefixes, like the
// link preview, because it says nothing in any language.
const ScriptPath = "/assets/webauthn.js"

// webauthnScript is the script, read once, and the tag that names this
// build's copy of it.
var webauthnScript, webauthnETag = func() ([]byte, string) {
	body, err := web.Assets.ReadFile("assets/webauthn.js")
	if err != nil {
		panic("httpx: the WebAuthn script is not embedded: " + err.Error())
	}
	sum := sha256.Sum256(body)
	return body, `"` + hex.EncodeToString(sum[:16]) + `"`
}()

// script serves the WebAuthn script. The browser asks again each time and is
// answered "not modified" while the build is the same, so a deployment never
// leaves a page running the previous build's script.
func script(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", webauthnETag)
	http.ServeContent(w, r, "webauthn.js", time.Time{}, bytes.NewReader(webauthnScript))
}
