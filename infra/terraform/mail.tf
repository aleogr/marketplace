# What the service needs to send e-mail, and to recognise what the provider
# posts back.
#
# The provider is Brevo (docs/requirements.md, section 25). Two credentials are
# involved and they are owned in opposite directions: the API key belongs to
# the provider's account and is put into Secret Manager by hand, so it never
# exists in this repository or in the Terraform state; the webhook token is
# this platform's own and is generated here, because nobody needs to choose it.

# The secret the API key is put into.
#
# Terraform owns the container and not the value: a key written into a variable
# would be in the state, in a plan output, and in whatever pasted it there. The
# owner adds the version with one command, and that command is the only place
# the key is ever handled (docs/roadmap.md, F11).
resource "google_secret_manager_secret" "mail_api_key" {
  secret_id = "mail-api-key"
  labels    = local.labels

  replication {
    auto {}
  }
}

# The service reads it at run time, and the job does too, because the delivery
# probe is sent by the job (`send-probe`, infra/terraform/migrate_job.tf).
resource "google_secret_manager_secret_iam_member" "service_mail_api_key" {
  secret_id = google_secret_manager_secret.mail_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.service.email}"
}

resource "google_secret_manager_secret_iam_member" "migrator_mail_api_key" {
  secret_id = google_secret_manager_secret.mail_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.migrator.email}"
}

# The shared token the provider sends back with every event it posts.
#
# Brevo does not sign its webhooks, so this token is all that stands in front
# of the endpoint — which is why what arrives through it is re-read from the
# provider's API before the platform acts on it
# (internal/platform/mail.Mailer.Apply). The owner copies the value from Secret
# Manager into the provider's console; it is generated here so that nobody has
# to invent it and nobody has to store it anywhere else.
resource "random_password" "mail_webhook_token" {
  length  = 48
  special = false
}

resource "google_secret_manager_secret" "mail_webhook_token" {
  secret_id = "mail-webhook-token"
  labels    = local.labels

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "mail_webhook_token" {
  secret      = google_secret_manager_secret.mail_webhook_token.id
  secret_data = random_password.mail_webhook_token.result
}

resource "google_secret_manager_secret_iam_member" "service_mail_webhook_token" {
  secret_id = google_secret_manager_secret.mail_webhook_token.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.service.email}"
}

locals {
  # The address the provider posts its events to. It is an output rather than a
  # thing to remember, because it is typed into the provider's console by hand.
  mail_webhook_url = "https://${var.platform_host}/webhooks/email/${var.providers_mode == "real" ? "brevo" : "mailbox"}"
}
