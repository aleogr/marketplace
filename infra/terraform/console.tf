# The console's first run (docs/superpowers/specs/2026-09-26-f15-console-design.md, D2).
#
# While no owner exists, the console serves /setup, and whoever presents this
# token there creates the owner. It is generated here, the way the mail
# webhook token is (infra/terraform/mail.tf), rather than by the service:
# the token never appears in a log, the runtime account gets no permission to
# write secrets, and every instance reads the same one. The owner reads it
# once, with the command in docs/infrastructure.md; the service records its
# SHA-256 when it is used, and /setup is "not found" from then on.
#
# 48 letters and digits are about 285 bits, beyond the 32 bytes of entropy
# the spec asks for; the service refuses a token shorter than 32 characters
# (internal/platform/config.MinBootstrapToken).
resource "random_password" "bootstrap_token" {
  length  = 48
  special = false
}

resource "google_secret_manager_secret" "bootstrap_token" {
  secret_id = "console-bootstrap-token"
  labels    = local.labels

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "bootstrap_token" {
  secret      = google_secret_manager_secret.bootstrap_token.id
  secret_data = random_password.bootstrap_token.result
}

# The service reads it, and nothing more: secretAccessor on this one secret.
# The migration job never needs it.
resource "google_secret_manager_secret_iam_member" "service_bootstrap_token" {
  secret_id = google_secret_manager_secret.bootstrap_token.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.service.email}"
}
