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

# The chains are walked once a day, and a break is a failed job.
#
# Tamper evidence that nobody looks at is not evidence: the property the hash
# chain gives is that an alteration is *detectable*, and this is what detects it
# (docs/requirements.md, section 21). Daily rather than by the minute because a
# chain that broke is a thing to investigate, not a thing to catch in the act —
# and walking every record costs a read of the whole log.
resource "google_cloud_scheduler_job" "verify_audit_chain" {
  name             = "verify-audit-chain"
  description      = "Recomputes every audit chain and fails if a record does not verify."
  schedule         = "17 4 * * *"
  time_zone        = "Etc/UTC"
  region           = var.region
  attempt_deadline = "600s"

  retry_config {
    retry_count = 1
  }

  http_target {
    uri         = "https://${var.platform_host}/internal/tasks"
    http_method = "POST"
    headers     = { "Content-Type" = "application/json" }
    body        = base64encode(jsonencode({ job = "verify-audit-chain" }))

    oidc_token {
      service_account_email = google_service_account.invoker.email
      audience              = "https://${var.platform_host}"
    }
  }

  depends_on = [google_project_service.scheduler]
}

# The migration job audits what it declares, in the transaction that declares
# it, so it needs the same key the service uses (cmd/marketplace, `migrate`).
resource "google_kms_crypto_key_iam_member" "migrator" {
  crypto_key_id = google_kms_crypto_key.audit.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.migrator.email}"
}
