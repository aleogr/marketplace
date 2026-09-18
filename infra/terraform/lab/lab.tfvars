# Passed to `terraform plan|apply -var-file=lab/lab.tfvars`. project_id is not
# here: it comes from the repository variable GCP_PROJECT_ID, so moving the lab
# to another project changes a setting rather than this repository.
region      = "us-central1"
environment = "lab"

# The platform host of the lab (docs/requirements.md, section 7).
platform_host = "marketplace.lab.aleogr.dev"

# The cheapest instance Cloud SQL offers, and the shortest recovery window it
# allows. Both are deliberate and both are recorded, with the risk each carries,
# in docs/infrastructure.md.
database_tier               = "db-f1-micro"
database_backup_count       = 7
database_log_retention_days = 1
