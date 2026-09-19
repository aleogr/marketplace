# The job that applies migrations.
#
# It runs the *same image* the service runs, under a different entry point:
# there is one binary, so there is no second image to build, no second thing to
# scan, and no way for the migrations to be from a different commit than the
# code that expects them (docs/roadmap.md, F4).
#
# The pipeline executes it before the new revision receives traffic
# (.github/workflows/deploy.yml). That order is the whole point: a revision must
# never meet a schema that has not been migrated yet.

resource "google_service_account" "migrator" {
  account_id   = "marketplace-migrate"
  display_name = "Cloud Run Job (database migrations)"
  description  = "Identity the migration job runs as. It may create and drop tables, which the service must never be able to do, and it is the only account that can read the database password."
}

resource "google_project_iam_member" "migrator_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.migrator.email}"
}

resource "google_cloud_run_v2_job" "migrate" {
  name     = "marketplace-migrate"
  location = var.region
  labels   = local.labels

  deletion_protection = false

  template {
    template {
      service_account = google_service_account.migrator.email

      # A migration that fails is not retried. Goose applies each migration in
      # its own transaction and records it, so a retry would be safe — but a
      # failed migration is something to read, not something to hope about, and
      # the pipeline stops on it either way.
      max_retries = 0
      timeout     = "600s"

      containers {
        image = var.initial_image
        args  = ["migrate"]

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        env {
          name  = "DATABASE_INSTANCE"
          value = google_sql_database_instance.main.connection_name
        }

        env {
          name  = "DATABASE_NAME"
          value = google_sql_database.marketplace.name
        }

        env {
          name  = "DATABASE_USER"
          value = google_sql_user.migrator.name
        }

        # Cloud Run reads the secret and sets the variable; nothing in this
        # repository or in a workflow ever holds the value.
        env {
          name = "DATABASE_PASSWORD"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.migrator_password.secret_id
              version = "latest"
            }
          }
        }

        # The user the migrations grant privileges to: the service's IAM
        # database user, which Cloud SQL creates without any rights at all.
        env {
          name  = "DATABASE_APP_USER"
          value = google_sql_user.service.name
        }

        # The marketplaces this environment declares, as the job reads them.
        # Terraform is what knows them, because it is also what maps their
        # hosts; the database learns them from here rather than from a
        # migration every environment would run (internal/tenancy/seed.go).
        env {
          name  = "SEED_MARKETPLACES"
          value = jsonencode(var.marketplaces)
        }

        env {
          name  = "LOG_LEVEL"
          value = "info"
        }

        # The job is also how one e-mail is sent by hand, to prove a sending
        # domain works: the same image, a different entry point
        # (`gcloud run jobs execute ... --args=send-probe,<address>`). It
        # therefore reads what sending needs (docs/roadmap.md, F11).
        env {
          name  = "PROVIDERS_MODE"
          value = var.providers_mode
        }

        # Declared only where there is one, for the reason the service's own
        # declaration gives (infra/terraform/cloud_run.tf): a variable set to
        # nothing refuses the start.
        dynamic "env" {
          for_each = var.mail_from != "" ? [1] : []
          content {
            name  = "MAIL_FROM"
            value = var.mail_from
          }
        }

        env {
          name  = "MAIL_DIRECTORY"
          value = "/tmp/mailbox"
        }

        dynamic "env" {
          for_each = var.providers_mode == "real" ? [1] : []
          content {
            name = "MAIL_API_KEY"
            value_source {
              secret_key_ref {
                secret  = google_secret_manager_secret.mail_api_key.secret_id
                version = "latest"
              }
            }
          }
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [
      # The pipeline sets this on every merge, by digest, exactly as it does
      # for the service. Nothing Terraform declares may depend on an image that
      # is not deployed yet (docs/infrastructure.md).
      template[0].template[0].containers[0].image,
      client,
      client_version,
    ]
  }

  depends_on = [
    google_secret_manager_secret_iam_member.migrator_password,
    google_secret_manager_secret_iam_member.migrator_mail_api_key,
  ]
}

# The pipeline rolls the job forward and executes it, on this job alone.
resource "google_cloud_run_v2_job_iam_member" "deployer_runs_migrations" {
  project  = var.project_id
  location = google_cloud_run_v2_job.migrate.location
  name     = google_cloud_run_v2_job.migrate.name
  role     = "roles/run.developer"
  member   = "serviceAccount:${google_service_account.deployer.email}"
}

# Updating a job means naming the identity it runs as, which Google requires
# this role on. Granted on the migration account alone, as it is on the
# service's (infra/terraform/deployer.tf).
resource "google_service_account_iam_member" "deployer_acts_as_migrator" {
  service_account_id = google_service_account.migrator.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer.email}"
}
