# Infrastructure

Reference for the Google Cloud project behind the platform, and the record of
the steps performed by hand.

> **Everything that can be declared is declared in Terraform** (`infra/terraform/`).
> This document covers only what cannot be: the resources that must exist before
> Terraform can hold its own state, and the settings that live in a provider's
> console rather than in this repository.

---

## Environments

| Environment | Project | State |
|---|---|---|
| lab | `aleogr-marketplace-lab-a4j5` | in use |
| production | not created | planned; see `docs/roadmap.md`, section 8 |

The project **number** of the lab is `1084000440884`. It appears inside the
Workload Identity Federation principal and cannot be replaced by the project id
there.

### Naming

Project ids are **immutable and cannot be reused after deletion**, so everything
that cannot be fixed later is in the name from the start:

```
aleogr  -  marketplace  -  lab   -  a4j5
  ^           ^             ^         ^
owner      product     environment  random suffix
```

- The **environment** is in the id because a project serves exactly one, and a
  project cannot be renamed into another.
- The **random suffix** is there because project ids are globally unique and a
  deleted id is gone for good; four characters keep `-prod` comfortably inside
  the 30-character limit.
- The **display name** (`marketplace-lab`) is mutable and carries no suffix.

Production will therefore be `aleogr-marketplace-prod-a4j5`, with the same
suffix: its job is uniqueness, not telling siblings apart.

## Billing

The project is on a **paid** Cloud Billing account. This is a prerequisite, not
a detail: a free trial stops every resource in the project when the trial ends,
and the trial can be neither paused nor extended, so a Cloud SQL instance
created under one has an expiry date rather than a lifetime.

## Bootstrap performed by hand

Run once, in Cloud Shell, because Terraform cannot create the bucket that holds
its own state, nor the identity it authenticates with. Everything else is
Terraform's.

The owner has no local development environment (`docs/requirements.md`,
section 27) and a Claude Code session has no `gcloud`, so these are Cloud Shell
commands by necessity, not by preference.

```bash
bash <<'BOOTSTRAP'
set -e

PROJECT_ID=aleogr-marketplace-lab-a4j5
REGION=us-central1
GITHUB_REPO=aleogr/marketplace
STATE_BUCKET=${PROJECT_ID}-tfstate
POOL=github
PROVIDER=marketplace-ci
SA_NAME=terraform
SA_EMAIL=${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com

gcloud services enable \
  serviceusage.googleapis.com cloudresourcemanager.googleapis.com \
  iam.googleapis.com iamcredentials.googleapis.com sts.googleapis.com \
  run.googleapis.com sqladmin.googleapis.com artifactregistry.googleapis.com \
  secretmanager.googleapis.com cloudkms.googleapis.com \
  cloudtasks.googleapis.com cloudscheduler.googleapis.com \
  storage.googleapis.com monitoring.googleapis.com logging.googleapis.com \
  --project="$PROJECT_ID"

gcloud storage buckets create "gs://${STATE_BUCKET}" \
  --project="$PROJECT_ID" --location="$REGION" \
  --uniform-bucket-level-access --public-access-prevention
gcloud storage buckets update "gs://${STATE_BUCKET}" --versioning

gcloud iam service-accounts create "$SA_NAME" \
  --project="$PROJECT_ID" --display-name="Terraform (CI)"

for ROLE in \
  roles/run.admin roles/cloudsql.admin roles/artifactregistry.admin \
  roles/secretmanager.admin roles/cloudkms.admin \
  roles/cloudtasks.admin roles/cloudscheduler.admin roles/storage.admin \
  roles/monitoring.admin roles/logging.admin \
  roles/serviceusage.serviceUsageAdmin \
  roles/iam.serviceAccountAdmin roles/iam.serviceAccountUser \
  roles/resourcemanager.projectIamAdmin
do
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" --role="$ROLE" \
    --condition=None --quiet > /dev/null
done

gcloud iam workload-identity-pools create "$POOL" \
  --project="$PROJECT_ID" --location=global --display-name="GitHub Actions"

gcloud iam workload-identity-pools providers create-oidc "$PROVIDER" \
  --project="$PROJECT_ID" --location=global --workload-identity-pool="$POOL" \
  --display-name="marketplace CI" \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
  --attribute-condition="assertion.repository=='${GITHUB_REPO}'"

PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')

gcloud iam service-accounts add-iam-policy-binding "$SA_EMAIL" \
  --project="$PROJECT_ID" --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL}/attribute.repository/${GITHUB_REPO}"
BOOTSTRAP
```

### Why each part is what it is

- **The state bucket has object versioning.** A corrupted or truncated state
  file is recoverable from the previous generation; without versioning it is
  recovered by rebuilding the environment.
- **The roles are an explicit list, not `roles/owner`.** A missing permission
  then fails a `terraform apply` naming the permission, which is a cheap and
  specific failure. `roles/owner` would never fail and would hand CI the
  authority to delete the project.
- **`--attribute-condition` is mandatory, and not only because Google requires
  it.** A GitHub OIDC provider without one accepts a token from *any* repository
  on GitHub, belonging to anyone. The condition is what makes the federation
  mean "this repository" rather than "GitHub told me who this is".
- **No service account key is created anywhere.** GitHub exchanges a short-lived
  OIDC token for credentials on each run, so there is nothing to leak from a
  public repository (`docs/requirements.md`, section 27).

## GitHub repository variables

Set under **Settings → Secrets and variables → Actions → Variables**. They are
**variables, not secrets**, on purpose: none is a credential, and a variable is
readable in the workflow log, so a wrong value fails with a message instead of
with `***`.

| Name | Value |
|---|---|
| `GCP_PROJECT_ID` | `aleogr-marketplace-lab-a4j5` |
| `GCP_TERRAFORM_SA` | `terraform@aleogr-marketplace-lab-a4j5.iam.gserviceaccount.com` |
| `GCP_WIF_PROVIDER` | `projects/1084000440884/locations/global/workloadIdentityPools/github/providers/marketplace-ci` |
| `TF_STATE_BUCKET` | `aleogr-marketplace-lab-a4j5-tfstate` |

The billing account id is deliberately **not** recorded here: the repository is
public and it identifies a payment instrument.

## Branch protection

The `main` branch is covered by the `protect-main` ruleset: a pull request is
required, the four CI checks must pass, branches must be up to date before
merging, deletions are restricted and force pushes are blocked.

Required approvals is **0**, because Claude Code never approves its own pull
requests and there is one human on the repository; the guarantee comes from the
required checks, not from an approval count that one person cannot supply.

## Replaying this for production

The block above is the procedure, with `PROJECT_ID` changed and the environment
part of the name moved to `prod`. Two things differ:

1. A **folder** per environment becomes worth creating once there is more than
   one project, so that IAM and organisation policy can differ between lab and
   production without repeating bindings project by project.
2. The production domain approach is an open question (`docs/design.md`,
   section 7): Cloud Run domain mapping is a preview feature, and the spike
   comparing it with a load balancer and with Cloudflare as a proxy is scheduled
   before launch.
