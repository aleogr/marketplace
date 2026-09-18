# The identity the deployment pipeline uses, which is not the identity
# Terraform uses.
#
# The Terraform account may create and destroy the project's infrastructure;
# the deploy account may push an image and roll the service forward, and
# nothing else. They are separate because they run on different triggers and
# fail differently: a mistake in a deployment should not be able to delete a
# database (docs/requirements.md, section 26).
#
# Like the Terraform account, it holds no key. GitHub exchanges a short-lived
# OIDC token for credentials on each run through the federation the bootstrap
# block created (docs/infrastructure.md).

resource "google_service_account" "deployer" {
  account_id   = "deployer"
  display_name = "Deployment pipeline (CI)"
  description  = "Builds, pushes and rolls forward the Cloud Run service from GitHub Actions. It cannot create or destroy infrastructure; that is the Terraform account's."
}

# Terraform does not manage the federation pool — it cannot, because the pool
# is what authenticates Terraform itself — but it does decide which accounts
# the pool may impersonate. The principal set is the same shape the bootstrap
# block used: this repository, and no other, whoever else federates into this
# pool later.
data "google_project" "current" {
  project_id = var.project_id
}

resource "google_service_account_iam_member" "deployer_federation" {
  service_account_id = google_service_account.deployer.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${data.google_project.current.number}/locations/global/workloadIdentityPools/${var.workload_identity_pool}/attribute.repository/${var.github_repository}"
}

# Push images, and only to this repository. The role is granted on the registry
# rather than on the project, so a second registry added later is not writable
# by accident.
resource "google_artifact_registry_repository_iam_member" "deployer_push" {
  project    = var.project_id
  location   = google_artifact_registry_repository.containers.location
  repository = google_artifact_registry_repository.containers.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.deployer.email}"
}

# Roll the service forward, and only this service. `run.developer` on the
# service itself can create a revision and move traffic to it; it cannot create
# a service, delete one, or touch anything else in the project.
resource "google_cloud_run_v2_service_iam_member" "deployer_deploy" {
  project  = var.project_id
  location = google_cloud_run_v2_service.marketplace.location
  name     = google_cloud_run_v2_service.marketplace.name
  role     = "roles/run.developer"
  member   = "serviceAccount:${google_service_account.deployer.email}"
}

# Deploying a revision means saying which identity it runs as, and Google
# requires the deployer to hold this role on that identity. It is granted on
# the runtime account alone: project-wide, it would let the pipeline run code
# as any account in the project, including Terraform's.
resource "google_service_account_iam_member" "deployer_acts_as_service" {
  service_account_id = google_service_account.service.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer.email}"
}
