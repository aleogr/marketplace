// Security keys and the device's own authenticators (WebAuthn), for the forms
// that carry data-webauthn: "create" adds one, "get" answers with one. The
// options come from the server in data-options; the browser's answer goes
// back in the form's hidden field and the form is posted as any other
// (docs/superpowers/specs/2026-09-25-f14-two-factor-design.md). This is the
// only script the site serves, loaded from this origin under the page's
// nonce; nothing here is inline.
"use strict";

(() => {
  // WebAuthn speaks bytes; JSON carries them as base64url without padding.
  const toBytes = (text) => {
    const base64 = text.replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    return Uint8Array.from(atob(padded), (c) => c.charCodeAt(0));
  };
  const toText = (buffer) => {
    let binary = "";
    for (const byte of new Uint8Array(buffer)) {
      binary += String.fromCharCode(byte);
    }
    return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  };

  const creation = (options) => {
    const publicKey = options.publicKey;
    publicKey.challenge = toBytes(publicKey.challenge);
    publicKey.user.id = toBytes(publicKey.user.id);
    for (const credential of publicKey.excludeCredentials || []) {
      credential.id = toBytes(credential.id);
    }
    return { publicKey };
  };
  const request = (options) => {
    const publicKey = options.publicKey;
    publicKey.challenge = toBytes(publicKey.challenge);
    for (const credential of publicKey.allowCredentials || []) {
      credential.id = toBytes(credential.id);
    }
    return { publicKey };
  };

  const created = (credential) =>
    JSON.stringify({
      id: credential.id,
      rawId: toText(credential.rawId),
      type: credential.type,
      authenticatorAttachment: credential.authenticatorAttachment || undefined,
      clientExtensionResults: credential.getClientExtensionResults(),
      response: {
        attestationObject: toText(credential.response.attestationObject),
        clientDataJSON: toText(credential.response.clientDataJSON),
        transports: credential.response.getTransports ? credential.response.getTransports() : [],
      },
    });
  const asserted = (credential) =>
    JSON.stringify({
      id: credential.id,
      rawId: toText(credential.rawId),
      type: credential.type,
      clientExtensionResults: credential.getClientExtensionResults(),
      response: {
        authenticatorData: toText(credential.response.authenticatorData),
        clientDataJSON: toText(credential.response.clientDataJSON),
        signature: toText(credential.response.signature),
        userHandle: credential.response.userHandle ? toText(credential.response.userHandle) : undefined,
      },
    });

  // The page's own messages, in its language: one for a browser that has no
  // WebAuthn at all, one for a ceremony the person cancelled or the browser
  // could not complete. A message is hidden and shown again only after a
  // frame has been drawn without it, so that a second failure is announced
  // again rather than left as an alert already read.
  const show = (id) => {
    const message = document.getElementById(id);
    if (message) {
      message.hidden = true;
      requestAnimationFrame(() =>
        requestAnimationFrame(() => {
          message.hidden = false;
        }),
      );
    }
  };
  const unsupported = () => show("key-unsupported");
  const failed = () => show("key-failed");

  for (const form of document.querySelectorAll("form[data-webauthn]")) {
    const button = form.querySelector("button[type=submit]");
    // The page hides the button, so that nothing posts an empty answer, and
    // spends an attempt, where no key can answer: without this script, or in
    // a browser without WebAuthn, which is told so here instead of being
    // asked to try again.
    if (window.PublicKeyCredential) {
      button.hidden = false;
    } else {
      unsupported();
    }
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      if (!window.PublicKeyCredential) {
        unsupported();
        return;
      }
      button.disabled = true;
      try {
        const options = JSON.parse(form.dataset.options);
        if (form.dataset.webauthn === "create") {
          const credential = await navigator.credentials.create(creation(options));
          form.elements.namedItem("credential").value = created(credential);
        } else {
          const credential = await navigator.credentials.get(request(options));
          form.elements.namedItem("key").value = asserted(credential);
        }
        form.submit();
      } catch {
        // Disabling the button took the focus away from it: it goes back,
        // so a keyboard or a screen reader is where the person left it.
        button.disabled = false;
        button.focus();
        failed();
      }
    });
  }
})();
