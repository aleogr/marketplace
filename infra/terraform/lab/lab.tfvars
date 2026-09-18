# Passed to `terraform plan|apply -var-file=lab/lab.tfvars`. project_id is not
# here: it comes from the repository variable GCP_PROJECT_ID, so moving the lab
# to another project changes a setting rather than this repository.
region      = "us-central1"
environment = "lab"

# The platform host of the lab (docs/requirements.md, section 7).
platform_host = "marketplace.lab.aleogr.dev"
