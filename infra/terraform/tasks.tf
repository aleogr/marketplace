# Work that must not happen inside a request.
#
# Cloud Run scales to zero, so there is no worker to hand work to and nothing
# to loop in. Cloud Tasks holds the work and calls this same service back;
# Cloud Scheduler does the same for the periodic jobs (docs/design.md, section
# 2.5). Both stay inside the free allowance at this volume: Cloud Tasks bills
# nothing for the first million operations a month, and Cloud Scheduler allows
# three jobs per billing account.

resource "google_project_service" "tasks" {
  service            = "cloudtasks.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "scheduler" {
  service            = "cloudscheduler.googleapis.com"
  disable_on_destroy = false
}

# One queue per class of work, because a flood of notifications must not delay
# a payment webhook. The retry policies differ for the same reason: a webhook
# to a gateway is worth trying for hours, a notification is not worth trying
# forever.
locals {
  queues = {
    webhooks = {
      max_attempts       = 10
      max_retry_duration = "3600s"
      max_dispatches     = 10
    }
    notifications = {
      max_attempts       = 5
      max_retry_duration = "1800s"
      max_dispatches     = 20
    }
    jobs = {
      max_attempts       = 3
      max_retry_duration = "600s"
      # Jobs take a lock keyed by name, so more than a few at once only
      # produces runs that stand down (internal/platform/jobs).
      max_dispatches = 5
    }
  }
}

resource "google_cloud_tasks_queue" "work" {
  for_each = local.queues

  name     = each.key
  location = var.region

  rate_limits {
    max_dispatches_per_second = each.value.max_dispatches
    # One attempt of one task at a time per queue is not needed; what must not
    # overlap is a job, and that is the lock's job rather than the queue's.
    max_concurrent_dispatches = each.value.max_dispatches
  }

  retry_config {
    max_attempts       = each.value.max_attempts
    max_retry_duration = each.value.max_retry_duration
    min_backoff        = "1s"
    max_backoff        = "300s"
    # Each retry waits twice as long, up to the maximum: a provider that is
    # down is not helped by being asked ten times a second.
    max_doublings = 5
  }

  depends_on = [google_project_service.tasks]
}

# The identity the callbacks are signed as.
#
# It is not the service's own account: the service calls nothing, it answers.
# A separate identity is what lets the endpoint refuse everything else, since a
# valid Google token proves who is calling and not that they may
# (internal/platform/httpx.Tasks).
resource "google_service_account" "invoker" {
  account_id   = "marketplace-invoker"
  display_name = "Cloud Tasks and Cloud Scheduler"
  description  = "Signs the callbacks that Cloud Tasks and Cloud Scheduler make into the service. It may invoke the service and nothing else."
}

resource "google_cloud_run_v2_service_iam_member" "invoker" {
  project  = var.project_id
  location = google_cloud_run_v2_service.marketplace.location
  name     = google_cloud_run_v2_service.marketplace.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.invoker.email}"
}

# The service's own account may create tasks, and only in this project's
# queues.
resource "google_project_iam_member" "service_enqueues" {
  project = var.project_id
  role    = "roles/cloudtasks.enqueuer"
  member  = "serviceAccount:${google_service_account.service.email}"
}

# Creating a task that runs as the invoker means acting as it.
resource "google_service_account_iam_member" "service_acts_as_invoker" {
  service_account_id = google_service_account.invoker.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.service.email}"
}

# The outbox is emptied on a schedule rather than on the way out of a request:
# a visitor whose request also dispatched everybody's events would pay for
# everybody's work (cmd/marketplace, DispatchJob).
resource "google_cloud_scheduler_job" "dispatch_outbox" {
  name             = "dispatch-outbox"
  description      = "Hands the events waiting in the outbox to their queues."
  schedule         = "* * * * *"
  time_zone        = "Etc/UTC"
  region           = var.region
  attempt_deadline = "60s"

  retry_config {
    retry_count = 1
  }

  http_target {
    uri         = "https://${var.platform_host}/internal/tasks"
    http_method = "POST"
    headers     = { "Content-Type" = "application/json" }
    body        = base64encode(jsonencode({ job = "dispatch-outbox" }))

    oidc_token {
      service_account_email = google_service_account.invoker.email
      audience              = "https://${var.platform_host}"
    }
  }

  depends_on = [google_project_service.scheduler]
}
