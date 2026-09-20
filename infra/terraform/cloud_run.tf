# The lab's only workload.
#
# Terraform owns the service's *shape*: the identity it runs as, how it scales,
# every environment variable it reads, and the host it answers on. It
# deliberately does not own the *image*, which changes on every merge and
# belongs to the pipeline (.github/workflows/deploy.yml). Splitting it that way
# is what lets a deployment be a deployment rather than an infrastructure
# change, and what keeps `terraform plan` from reporting a difference after
# every merge.

resource "google_service_account" "service" {
  account_id   = "marketplace-run"
  display_name = "Cloud Run (marketplace service)"
  description  = "Identity the marketplace service runs as. Separate from the deploy identity so that what the running code may do and what the pipeline may do are two different answers (docs/requirements.md, section 26)."
}

# Everything the container writes to stdout becomes a log entry under the
# service's own identity, so the runtime account needs this one role and, for
# now, nothing else. Each later permission arrives with the delivery that needs
# it: the database connection with F4, the bucket with the media work.
resource "google_project_iam_member" "service_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.service.email}"
}

resource "google_cloud_run_v2_service" "marketplace" {
  name        = "marketplace"
  location    = var.region
  description = "The marketplace platform (docs/design.md, section 3)."
  labels      = local.labels

  # The name this service also answers to, for the tokens Cloud Tasks and
  # Cloud Scheduler sign their callbacks with. Without it, Cloud Run would
  # refuse a token minted for the platform's host, and the generated `run.app`
  # address cannot be named here because it does not exist until this resource
  # does (infra/terraform/tasks.tf).
  custom_audiences = ["https://${var.platform_host}"]

  # The service is the public web site; there is no load balancer in front of
  # it (docs/requirements.md, section 24), so it takes traffic directly.
  ingress = "INGRESS_TRAFFIC_ALL"

  # The lab is rebuilt from this configuration in minutes and holds nothing of
  # its own. What must not be lost by accident is the Terraform state, and that
  # is protected where it lives (docs/infrastructure.md).
  deletion_protection = false

  template {
    service_account = google_service_account.service.email

    # Go serves concurrent requests on one process, which is the whole reason
    # scaling to zero is affordable here: one warm instance absorbs the lab's
    # traffic instead of one instance per request (docs/design.md, section 3).
    max_instance_request_concurrency = 80

    # A request that has not finished in a minute is not going to. The default
    # is five, which mostly serves to keep a stuck instance billable.
    timeout = "60s"

    scaling {
      # Zero is the premise, not a tuning choice: the lab costs nothing while
      # nobody is looking at it (docs/requirements.md, section 26).
      min_instance_count = 0
      # A ceiling, not a capacity plan. It bounds what a loop or a crawler can
      # spend before anyone notices.
      max_instance_count = 2
    }

    containers {
      image = var.initial_image

      ports {
        container_port = 8080
      }

      resources {
        # The smallest allocation Cloud Run offers that still starts a Go
        # binary without a warm-up penalty.
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        # Outside a request the instance is idle and not billed for CPU, which
        # is what makes a warm instance cheap rather than a fixed cost.
        cpu_idle          = true
        startup_cpu_boost = true
      }

      # Every variable the process reads is declared here with an explicit
      # value, including the ones whose value equals the default
      # (docs/requirements.md, section 7.1). A key that exists only as an
      # absence is a key nobody finds on the day it matters.
      #
      # PORT is the exception, and not an omission: Cloud Run injects it from
      # `container_port` above and rejects a service that sets it by hand.
      env {
        name = "INDEXABLE"
        # False until the day the platform launches, when it becomes a single
        # change here. An indexed lab costs a domain migration and is invisible
        # for months (docs/requirements.md, section 7.1).
        value = "false"
      }

      env {
        name  = "LOG_LEVEL"
        value = "info"
      }

      # How the service reaches the database. There is no password here and no
      # secret mounted: the connector authenticates this revision's own service
      # account with IAM (infra/terraform/cloud_sql.tf).
      env {
        name  = "DATABASE_INSTANCE"
        value = local.database_instance
      }

      env {
        name  = "DATABASE_NAME"
        value = google_sql_database.marketplace.name
      }

      env {
        name  = "DATABASE_USER"
        value = google_sql_user.service.name
      }

      # The platform's own host. It is configuration rather than a row because
      # it belongs to the deployment and exists before any marketplace does
      # (docs/requirements.md, section 7).
      env {
        name  = "PLATFORM_HOST"
        value = var.platform_host
      }

      # Where the queues are, and who the callbacks are signed as. Declared
      # here rather than discovered at run time: a process that had to ask
      # which project it is in would fail differently in every environment
      # (infra/terraform/tasks.tf).
      env {
        name  = "TASKS_PROJECT"
        value = var.project_id
      }

      env {
        name  = "TASKS_LOCATION"
        value = var.region
      }

      env {
        name  = "TASKS_INVOKER"
        value = google_service_account.invoker.email
      }

      # The address Cloud Tasks and Cloud Scheduler call back, and which the
      # token must be minted for.
      #
      # The platform's own host rather than the generated `run.app` one: that
      # address is only known after the service exists, and naming it here
      # would be a resource referring to itself. Cloud Run accepts a token
      # minted for it because it is declared as a custom audience below —
      # which is the supported way to say "this service also answers to this
      # name".
      env {
        name  = "SERVICE_URL"
        value = "https://${var.platform_host}"
      }

      env {
        name = "PROVIDERS_MODE"
        # `fake` until the provider accounts exist. Turning it to `real` is a
        # line in the environment's tfvars, and the credentials those adapters
        # read have to be in Secret Manager first (docs/roadmap.md, F11).
        value = var.providers_mode
      }

      # Where the fake e-mail adapter writes the messages it does not send.
      # Cloud Run gives the container a writable /tmp in memory, which is the
      # right lifetime for it: the messages of a revision are the revision's.
      env {
        name  = "MAIL_DIRECTORY"
        value = "/tmp/mailbox"
      }

      # The address mail is sent from, declared when this environment has one.
      #
      # Not declared empty, and that is not a departure from "every variable
      # this process reads is declared here" (docs/requirements.md, section
      # 7.1): a variable set to nothing is not a declaration, and this binary
      # refuses to start on one, by design. An environment on the fake adapter
      # sends from no address at all, because it sends nothing.
      dynamic "env" {
        for_each = var.mail_from != "" ? [1] : []
        content {
          name  = "MAIL_FROM"
          value = var.mail_from
        }
      }

      # The shared token the e-mail provider posts its events with. Cloud Run
      # reads the secret and sets the variable; nothing in this repository or
      # in a workflow ever holds the value (infra/terraform/mail.tf).
      env {
        name = "MAIL_WEBHOOK_TOKEN"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.mail_webhook_token.secret_id
            version = "latest"
          }
        }
      }

      # The key that wraps every person's audit key. The service may ask this
      # key to wrap and unwrap and cannot read it, which is what makes a
      # destroyed key final (infra/terraform/audit.tf).
      env {
        name  = "AUDIT_KEY"
        value = google_kms_crypto_key.audit.id
      }

      # The provider's API key, wired only where a provider is really talked
      # to. A secret with no version yet would refuse the revision, and until
      # the owner adds one there is nothing to read.
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

      # Traffic reaches a revision only once the process is listening. Without
      # this, a revision that refuses to start over a misread variable — which
      # is a designed behaviour here, not a fault — would take the service down
      # instead of failing the deployment. A process that refuses to start
      # never listens, so a connection to the port answers that question.
      #
      # It is deliberately a TCP check and not an HTTP path, and that is not a
      # simplification: Terraform applies *before* the image is deployed
      # (.github/workflows/deploy.yml), so the revision this probe first runs
      # against is the one already running. A probe naming an application route
      # therefore deadlocks the moment that route changes — the apply creates a
      # revision whose probe the old image fails, the apply fails, and the
      # deployment that would have brought the image serving the new route is
      # skipped because the apply failed. That happened once, on the merge that
      # moved the health check off /healthz. Nothing Terraform declares here may
      # depend on an image that does not exist yet.
      startup_probe {
        tcp_socket {
          port = 8080
        }
        period_seconds    = 3
        timeout_seconds   = 3
        failure_threshold = 10
      }

      # No liveness probe on purpose. It would answer "restart this instance",
      # and a process that holds no state and no background work has nothing a
      # restart repairs; what it would add is a restart loop on a slow
      # dependency. The delivery that gives the binary background work is the
      # one that should reconsider this.
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_secret_manager_secret_iam_member.service_mail_webhook_token,
    google_secret_manager_secret_iam_member.service_mail_api_key,
    google_kms_crypto_key_iam_member.service,
  ]

  lifecycle {
    ignore_changes = [
      # The pipeline sets this on every merge, by digest. Without the
      # exception, the next `terraform apply` would roll the service back to
      # the image named above and every plan would report a difference.
      template[0].containers[0].image,
      # Written by whichever client last updated the service, which is
      # `gcloud run deploy` and not Terraform.
      client,
      client_version,
    ]
  }
}

# The platform is a public web site. Who may do what inside it is the
# application's answer, decided by roles and permissions (docs/requirements.md,
# section 19); IAM's only job here is to let the request arrive.
resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = var.project_id
  location = google_cloud_run_v2_service.marketplace.location
  name     = google_cloud_run_v2_service.marketplace.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# Cloud Run's native domain mapping, with Cloudflare in "DNS only" mode
# (docs/requirements.md, section 7). It is a preview feature and is re-examined
# before production traffic; nothing in the design depends on how the request
# reaches the service, because the marketplace is resolved from the host the
# service receives, whatever put it there (docs/design.md, section 7).
#
# The mapping refuses to be created until the account that applies it — the
# Terraform account, not the deploy account — is a verified owner of the base
# domain in Search Console. That is a step outside this repository, recorded in
# docs/infrastructure.md.
resource "google_cloud_run_domain_mapping" "platform" {
  location = var.region
  name     = var.platform_host

  metadata {
    namespace = var.project_id
    labels    = local.labels
  }

  spec {
    route_name = google_cloud_run_v2_service.marketplace.name
  }
}

# One mapping per marketplace host.
#
# Domain mapping supports no wildcard, so each host is its own resource and its
# own certificate — which is why creating a marketplace includes an
# infrastructure step and why that step can take a day (docs/requirements.md,
# section 7). They all point at the one service: which marketplace serves a
# request is decided inside the process, from the host it receives
# (internal/tenancy).
resource "google_cloud_run_domain_mapping" "marketplace" {
  for_each = local.marketplace_hosts

  location = var.region
  name     = each.value

  metadata {
    namespace = var.project_id
    labels    = local.labels
  }

  spec {
    route_name = google_cloud_run_v2_service.marketplace.name
  }
}
