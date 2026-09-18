# Passed to `terraform init -backend-config=lab/backend.hcl`. The bucket is not
# here: it comes from the repository variable TF_STATE_BUCKET (see backend.tf).
prefix = "lab"
