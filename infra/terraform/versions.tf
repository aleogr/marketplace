# The Terraform release is pinned to a minor series rather than to an exact
# patch: the exact patch lives in .terraform-version, which both the CI
# workflow and `make terraform-deps` read, so the number exists in one place
# and a patch upgrade does not need a change here.
terraform {
  required_version = "~> 1.16.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.3"
    }
  }
}
