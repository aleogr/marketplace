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

      env {
        name = "PROVIDERS_MODE"
        # Every external provider is still a fake: no gateway, no shipping
        # account and no e-mail provider exists yet, and the deliveries that
        # add them flip this (docs/roadmap.md, F11 onwards).
        value = "fake"
      }

      # Traffic reaches a revision only once the process answers. Without this,
      # a revision that refuses to start over a misread variable — which is a
      # designed behaviour here, not a fault — would take the service down
      # instead of failing the deployment.
      startup_probe {
        http_get {
          path = "/health"
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
