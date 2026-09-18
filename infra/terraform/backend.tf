# The state bucket is the one thing Terraform cannot create for itself, so it
# is created by hand and recorded in docs/infrastructure.md. Its name is not
# written here either: it reaches CI as the repository variable
# TF_STATE_BUCKET, and `prefix` comes from the environment's backend file
# (infra/terraform/lab/backend.hcl). Both are supplied with -backend-config,
# which is what lets a second environment be a new directory of values rather
# than a second copy of this configuration — and what keeps two environments
# from writing to one state file.
terraform {
  backend "gcs" {}
}
