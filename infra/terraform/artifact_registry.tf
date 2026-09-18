# The registry the container image is pushed to on every merge into main, and
# promoted from by digest on a release (docs/requirements.md, section 27).
#
# Tags are mutable, which F3 decided once the pipeline existed to decide it
# against. The pipeline deploys by digest and never by tag, so a tag is a label
# for people reading the registry, not the thing a revision runs. Immutable
# tags would buy nothing for that and would cost a re-run: a merge build that
# is restarted pushes the same commit's tag a second time, and the push would
# fail on a repository that refuses it.
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
