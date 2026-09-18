# The database.
#
# It is the only resource in this configuration that holds state nothing else
# can rebuild, and the only material recurring cost of the design
# (docs/design.md, section 3). Both facts show up below: in the deletion
# protection, and in how small everything is.

resource "google_sql_database_instance" "main" {
  name             = "marketplace"
  database_version = "POSTGRES_16"
  region           = var.region

  # Terraform refuses to destroy this instance. It is not the same setting as
  # `settings.deletion_protection_enabled`, which refuses at the API; both are
  # on, because the two failure modes are different — a mistaken `destroy` here
  # and a mistaken deletion anywhere else — and neither is expensive to hold.
  deletion_protection = true

  settings {
    tier = var.database_tier
    # One zone. High availability doubles the bill to protect a lab that is
    # rebuilt from this repository in minutes (docs/infrastructure.md).
    availability_type = "ZONAL"
    edition           = "ENTERPRISE"
    user_labels       = local.labels

    deletion_protection_enabled = true

    disk_type = "PD_SSD"
    disk_size = var.database_disk_size
    # The disk grows on its own rather than filling up at three in the morning,
    # but not without a ceiling: growth is one-way — Cloud SQL never shrinks a
    # disk — so an unbounded limit turns one runaway migration into a permanent
    # bill.
    disk_autoresize       = true
    disk_autoresize_limit = var.database_disk_limit

    backup_configuration {
      enabled    = true
      start_time = "06:00" # UTC, roughly 03:00 in Brazil: after the day's work.

      # Point-in-time recovery is what turns "we have last night's backup" into
      # a recovery objective measured in minutes (docs/design.md, section 3).
      point_in_time_recovery_enabled = true
      # The window, not the feature, is what costs: write-ahead logs are
      # archived every five minutes whether or not anything happened. The lab
      # keeps the shortest window Cloud SQL Enterprise allows; production
      # carries the design's targets (docs/infrastructure.md).
      transaction_log_retention_days = var.database_log_retention_days

      backup_retention_settings {
        retained_backups = var.database_backup_count
        retention_unit   = "COUNT"
      }
    }

    maintenance_window {
      day          = 7 # Sunday
      hour         = 7 # UTC
      update_track = "stable"
    }

    ip_configuration {
      # The instance has a public address and **no authorized network**, which
      # is not the contradiction it reads as: with an empty authorization list
      # nothing on the internet can open a connection. The only way in is the
      # Cloud SQL connector, which authenticates with IAM and encrypts with
      # certificates the Cloud SQL API issues per connection.
      #
      # The alternative — private IP only — is not cheaper or simpler here: it
      # requires `private_network`, which requires a VPC with private services
      # access and the peering that goes with it. Cloud SQL refuses an instance
      # with neither. This design deliberately has no network of its own to
      # maintain (docs/design.md, section 3), so the choice is the connector.
      ipv4_enabled = true
      # Refuses an unencrypted connection outright, so a client that skips TLS
      # fails at the handshake rather than succeeding quietly.
      ssl_mode = "ENCRYPTED_ONLY"
    }

    database_flags {
      # Lets a service account authenticate as a database user with a token
      # instead of a password. It is what allows the running service to hold no
      # credential at all (docs/design.md, section 3).
      name  = "cloudsql.iam_authentication"
      value = "on"
    }

    insights_config {
      query_insights_enabled = true
      # Free, and the only way to see a slow query after the fact on an
      # instance this small.
      query_string_length = 1024
    }
  }
}

resource "google_sql_database" "marketplace" {
  name     = "marketplace"
  instance = google_sql_database_instance.main.name
  # The application never creates or drops databases; migrations own what is
  # inside this one.
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
  # Cloud SQL derives the database user name from the account's e-mail without
  # the domain, and refuses the full address here.
  name     = trimsuffix(google_service_account.service.email, ".gserviceaccount.com")
  instance = google_sql_database_instance.main.name
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
  name     = "migrator"
  instance = google_sql_database_instance.main.name
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
