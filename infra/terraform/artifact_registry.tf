# The registry the container image is pushed to on every merge into main, and
# promoted from by digest on a release (docs/requirements.md, section 27).
#
# Whether tags should be immutable is decided with the deployment pipeline in
# F3, not here: it is the pipeline that decides what a tag names and whether
# re-running a build may push the same tag again.
resource "google_artifact_registry_repository" "containers" {
  location      = var.region
  repository_id = "containers"
  description   = "Container images of the marketplace service"
  format        = "DOCKER"
  labels        = local.labels

  # Artifact Registry charges for storage beyond half a gigabyte, and a
  # pipeline that pushes an image per merge reaches that on its own
  # (docs/requirements.md, section 26). A KEEP policy always wins over a
  # DELETE one, so the first rule is what stops the other two from removing
  # the image a service is currently running.
  cleanup_policy_dry_run = false

  cleanup_policies {
    id     = "keep-recent-images"
    action = "KEEP"

    most_recent_versions {
      keep_count = 20
    }
  }

  cleanup_policies {
    id     = "delete-untagged-images"
    action = "DELETE"

    condition {
      tag_state  = "UNTAGGED"
      older_than = "604800s" # 7 days
    }
  }

  cleanup_policies {
    id     = "delete-old-images"
    action = "DELETE"

    condition {
      tag_state  = "ANY"
      older_than = "7776000s" # 90 days
    }
  }
}
