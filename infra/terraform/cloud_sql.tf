# This tenant's slice of the shared instance.
#
# The instance itself is owned by aleogr/shared-infra, which is where its
# deletion protection, its size and its backup schedule are declared. What is
# declared here is what belongs to this tenant and not to the instance: the
# marketplace's own database, the two users that log into it, and the secret
# the migration job needs to become one of them (docs/design.md, section 3).

resource "google_sql_database" "marketplace" {
  project  = var.shared_project_id
  name     = "marketplace"
  instance = var.shared_instance

  # ABANDON, NOT DELETE, and the default is DELETE. The instance's protection
  # does not reach here: dropping a database is not deleting an instance, and
  # a rename Terraform reads as a replacement would drop it with every row in
  # it while the protected instance stood.
  deletion_policy = "ABANDON"
}

# Two identities reach the database, and they are not the same.
#
# The service authenticates with IAM and holds no password. The migration job
# needs privileges the service must never have — creating and dropping tables —
# and it is the one that bootstraps the schema, which is a thing IAM alone
# cannot do: a freshly created IAM database user has no rights on anything, and
# granting rights requires a connection that already has them. That first
# connection is the built-in user below, and it is the only password in the
# system.

resource "google_sql_user" "service" {
  project = var.shared_project_id
  # Cloud SQL derives the database user name from the account's e-mail without
  # the domain, and refuses the full address here.
  name     = trimsuffix(google_service_account.service.email, ".gserviceaccount.com")
  instance = var.shared_instance
  type     = "CLOUD_IAM_SERVICE_ACCOUNT"
}

resource "random_password" "migrator" {
  length = 32
  # Cloud SQL accepts more, but every character class that needs escaping in a
  # connection string is a future outage in an error message nobody can read.
  special          = true
  override_special = "-_.~"
}

resource "google_sql_user" "migrator" {
  project = var.shared_project_id
  # The rename from `migrator`: a role belongs to the cluster, not to a
  # database, so on a shared instance every tenant's roles share one
  # namespace, and a role named for its function rather than its tenant is a
  # trap the day a third tenant arrives (docs/design.md, section 3).
  name     = "marketplace_migrator"
  instance = var.shared_instance
  password = random_password.migrator.result
}

# The password exists in two places: here, and in the Terraform state, which
# lives in a private, versioned bucket that only the Terraform account can read
# (docs/infrastructure.md). It is never in this repository, never in a
# workflow's environment, and never in a log: the migration job reads it from
# Secret Manager at run time, and nothing else is granted access to it.
resource "google_secret_manager_secret" "migrator_password" {
  secret_id = "migrator-password"
  labels    = local.labels

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "migrator_password" {
  secret      = google_secret_manager_secret.migrator_password.id
  secret_data = random_password.migrator.result
}

# What each identity may do with the instance.
#
# `cloudsql.client` opens a connection through the connector; `instanceUser` is
# what makes an IAM database user able to log in at all. The service holds both
# and no password. The migration job holds only `client`, because it logs in as
# the built-in user, and the one grant that lets it read that user's password.

resource "google_project_iam_member" "service_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.service.email}"
}

resource "google_project_iam_member" "service_sql_login" {
  project = var.project_id
  role    = "roles/cloudsql.instanceUser"
  member  = "serviceAccount:${google_service_account.service.email}"
}

resource "google_project_iam_member" "migrator_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.migrator.email}"
}

# On the secret itself, not on the project: the job may read this password and
# no other secret this project ever holds.
resource "google_secret_manager_secret_iam_member" "migrator_password" {
  secret_id = google_secret_manager_secret.migrator_password.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.migrator.email}"
}
