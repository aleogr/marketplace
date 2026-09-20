# Passed to `terraform plan|apply -var-file=lab/lab.tfvars`. project_id is not
# here: it comes from the repository variable GCP_PROJECT_ID, so moving the lab
# to another project changes a setting rather than this repository.
region      = "us-central1"
environment = "lab"

# The platform host of the lab (docs/requirements.md, section 7).
platform_host = "marketplace.lab.aleogr.dev"

# Real external providers, which today means one: e-mail
# (docs/roadmap.md, F11). Every adapter this variable governs reads its
# credential from Secret Manager, so a provider added later needs its secret
# in place before this stays `real`.
providers_mode = "real"

# The address the platform's own mail is sent from.
#
# `lab.aleogr.dev` rather than the platform host: that host is a CNAME to Cloud
# Run, and a name that is a CNAME cannot also carry the verification record an
# e-mail provider asks for. The domain is authenticated at the provider — DKIM
# on both selectors, DMARC — and its reputation is shared with whatever else
# sends from it, which is the cost of the lab not having a domain of its own.
# Production gets its own sending domain, which is this line and nothing else.
mail_from = "marketplace@lab.aleogr.dev"

# The lab databases of this project and of `schooling` share one instance,
# in a project neither of them owns (aleogr/shared-infra). This configuration
# declares its own database inside it and cannot change the instance.
shared_project_id = "aleogr-lab-shared-dacd"

# The lab's first marketplace. Its host is mapped by Terraform and seeded into
# the database by the migration job, from this one declaration.
marketplaces = [
  {
    slug                = "marketplace1"
    name                = "Marketplace 1"
    market              = "BR"
    revenue_model       = "commission"
    default_language    = "pt-BR"
    languages           = ["en-US", "pt-BR"]
    hosts               = ["marketplace1.marketplace.lab.aleogr.dev"]
    detect_contact_data = true
    reveal_contact      = false
  }
]
