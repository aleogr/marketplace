# No credentials are configured here. In CI, Workload Identity Federation
# writes a short-lived credential file and exports GOOGLE_APPLICATION_
# CREDENTIALS, which the provider picks up on its own; there is no key to
# store and none to leak from a public repository (docs/requirements.md,
# section 27).
provider "google" {
  project = var.project_id
  region  = var.region
}
