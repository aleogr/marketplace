# The key that protects what the audit log holds.
#
# Every person whose data appears in the log has a key of their own, kept in the
# database wrapped by this one. A deletion request destroys the person's key and
# the log stays whole — which only means anything because the key that wrapped
# it lives here, where this service can ask for an operation and never for the
# key itself (docs/requirements.md, sections 21 and 18.3).

resource "google_kms_key_ring" "audit" {
  name     = "audit"
  location = var.region

  # A key ring cannot be deleted, ever, in any project: Google keeps the name
  # taken. That is not a limitation to work around here — it is the property
  # that makes a key manager worth using, and it is recorded so that the next
  # person does not go looking for the delete button (docs/infrastructure.md).
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_kms_crypto_key" "audit" {
  name     = "audit-user-keys"
  key_ring = google_kms_key_ring.audit.id
  purpose  = "ENCRYPT_DECRYPT"

  # A new version every ninety days. Older versions stay able to decrypt what
  # they encrypted, so rotation costs nothing and limits how much is protected
  # by any one version.
  rotation_period = "7776000s"

  lifecycle {
    prevent_destroy = true
  }
}

# The service wraps and unwraps; it cannot read the key, create one, or destroy
# one. Least privilege here is not ceremony: the difference between this role
# and an administrative one is whether a compromised revision can erase the
# platform's ability to read its own audit log.
resource "google_kms_crypto_key_iam_member" "service" {
  crypto_key_id = google_kms_crypto_key.audit.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.service.email}"
}
