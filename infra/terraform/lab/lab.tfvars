# Passed to `terraform plan|apply -var-file=lab/lab.tfvars`. project_id is not
# here: it comes from the repository variable GCP_PROJECT_ID, so moving the lab
# to another project changes a setting rather than this repository.
region      = "us-central1"
environment = "lab"
