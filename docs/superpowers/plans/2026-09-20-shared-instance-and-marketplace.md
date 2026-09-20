# Shared Lab Database Instance — Plan 1: the home and the rehearsal

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the shared Cloud SQL instance `lab-postgres` in a project of its own, and move the `marketplace` lab database onto it — proving the whole path with data that one deployment rebuilds.

**Architecture:** A new repository owns the shared project and the instance and nothing else. Each tenant keeps owning its own database, roles and grants, declared in its own repository against the shared project — `google_sql_database` and `google_sql_user` both take `project` and `instance`, so no tenant needs rights over the instance itself.

**Tech Stack:** Terraform (version pinned in `infra/terraform/.terraform-version`), Google Cloud provider, GitHub Actions with Workload Identity Federation, Go 1.27 for this repository's tests.

**Spec:** `docs/superpowers/specs/2026-09-20-shared-database-instance-design.md`

## Global Constraints

- Everything versioned is written in **English**: code, comments, commit messages, pull request titles and bodies, documentation.
- **No secret is ever committed.** Secrets live in Secret Manager or in the repository's encrypted secrets. Both repositories here are public.
- **Terraform is only ever applied by CI.** A session never runs `terraform apply`.
- Claude Code **never approves and never merges**. The owner merges.
- Manual steps outside a session are given **one at a time**: the owner runs it, reports the result, and only then receives the next.
- Nothing in `codeschool-ing/schooling` is written from this session. Steps there are handed over as prompts for that project's own session.
- Instance: `POSTGRES_16`, edition `ENTERPRISE`, tier `db-f1-micro`, `ZONAL`, 10 GB `PD_SSD`, `disk_autoresize_limit = 50`, backup window `12:00` UTC, `point_in_time_recovery_enabled = true`, `transaction_log_retention_days = 7`, `retained_backups = 7`, `ipv4_enabled = true`, `ssl_mode = ENCRYPTED_ONLY`, database flag `cloudsql.iam_authentication = on`, `deletion_protection` and `settings.deletion_protection_enabled` both `true`.
- Names: project `aleogr-lab-shared-<4>`, instance `lab-postgres`, databases `marketplace` and `schooling`, roles `marketplace_migrator` and `schooling`.

## Out of scope for this plan

Phases 2–4 of the spec — moving `schooling`, the sleep schedule, and
decommissioning the old instances. Plan 2 covers them and is written **after**
this plan runs, because Phase 1 is a rehearsal and a rehearsal's purpose is to
discover what the path requires. Writing it now would be writing the risky half
from imagination.

The old `marketplace` instance is **stopped, not deleted**, at the end of this
plan. Deleting it belongs to Plan 2's cool-down.

## File structure

**New repository `aleogr/lab-shared`:**

| file | responsibility |
|---|---|
| `infra/terraform/versions.tf` | provider and Terraform version floors |
| `infra/terraform/providers.tf` | the Google provider, with `billing_project` stated |
| `infra/terraform/backend.tf` | `backend "gcs" {}`, configured by `-backend-config` |
| `infra/terraform/variables.tf` | `project_id`, `region`, instance sizing |
| `infra/terraform/services.tf` | the project services Terraform needs enabled |
| `infra/terraform/instance.tf` | `lab-postgres`, and nothing else |
| `infra/terraform/tenants.tf` | the IAM that lets each tenant's deployer declare its own database |
| `infra/terraform/deployer.tf` | this repository's own deploy identity |
| `infra/terraform/outputs.tf` | `connection_name`, for tenants to read |
| `infra/terraform/lab/backend.hcl` | the state prefix |
| `infra/terraform/lab/lab.tfvars` | the environment's values |
| `tools/check-instance.sh` | asserts the live instance matches the design |
| `.github/workflows/ci.yml` | plan on a pull request, apply on `main` |
| `README.md` | what this repository is and what it is not |
| `CLAUDE.md` | the working agreements, pointing at this repository's own rules |

**This repository (`aleogr/marketplace`):**

| file | change |
|---|---|
| `infra/terraform/variables.tf` | add `shared_project_id` and `shared_instance` |
| `infra/terraform/cloud_sql.tf` | the instance resource goes; the database, the users, the grants and the secret stay, addressed at the shared project |
| `infra/terraform/cloud_run.tf:114` | `DATABASE_INSTANCE` reads a local instead of the removed resource |
| `infra/terraform/migrate_job.tf:54` | the same |
| `infra/terraform/outputs.tf:28` | the same |
| `infra/terraform/locals.tf` | `local.database_instance`, the one place the connection name is assembled |
| `infra/terraform/lab/lab.tfvars` | the shared project's id |
| `cmd/marketplace/deployment_test.go` | a test that both surfaces name the **same** instance |
| `docs/infrastructure.md` | the decisions that survived |

---

### Task 1: The shared project exists

Owner-executed. Nothing in either repository changes.

**Files:** none.

**Interfaces:**
- Produces: the project id `aleogr-lab-shared-<4>` and the state bucket name, both of which every later task needs.

- [ ] **Step 1: Pick the suffix and create the project**

Hand the owner exactly this, one block, and wait for the output:

```sh
SUFFIX=$(openssl rand -hex 2)            # four characters, as the convention asks
PROJECT="aleogr-lab-shared-$SUFFIX"
echo "$PROJECT"

gcloud projects create "$PROJECT" \
  --name="lab shared" \
  --organization=421838980939

gcloud billing projects link "$PROJECT" --billing-account=0194C5-7DEBC1-C23BB3
```

Expected: the project id echoed, then `Operation ... finished successfully` twice.

- [ ] **Step 2: Enable only what Terraform itself needs**

```sh
gcloud services enable \
  cloudresourcemanager.googleapis.com \
  serviceusage.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  sqladmin.googleapis.com \
  --project="$PROJECT"
```

Expected: `Operation ... finished successfully.`

- [ ] **Step 3: Create the state bucket**

Terraform cannot create the place it stores its state, so this is by hand and
recorded in `docs/infrastructure.md` — the same reasoning as this repository's
own bucket (`infra/terraform/backend.tf`).

```sh
BUCKET="${PROJECT}-tfstate"
gcloud storage buckets create "gs://$BUCKET" \
  --project="$PROJECT" --location=us-central1 \
  --uniform-bucket-level-access --public-access-prevention
gcloud storage buckets update "gs://$BUCKET" --versioning
echo "$BUCKET"
```

Expected: `Creating gs://...` then the bucket name echoed. Versioning matters:
a truncated state write must be recoverable.

- [ ] **Step 4: Record both values**

The project id and the bucket name are inputs to every later task. Write them
into the plan's own notes when reporting back.

---

### Task 2: The repository exists and this session can write to it

**Files:**
- Create: `README.md`, `CLAUDE.md` in `aleogr/lab-shared`

**Interfaces:**
- Consumes: the project id from Task 1.
- Produces: a clone at `/home/user/lab-shared` this session can push to.

- [ ] **Step 1: Confirm the repository name with the owner**

Proposed: `aleogr/lab-shared`. Public, like the others, which is why the
no-secrets rule applies to it too.

- [ ] **Step 2: Owner creates it**

```sh
# In the browser, or with the GitHub CLI if the owner has it:
#   https://github.com/organizations/aleogr/repositories/new
#   name: lab-shared   visibility: public   no README, no .gitignore, no licence
```

Expected: an empty repository at `https://github.com/aleogr/lab-shared`.

- [ ] **Step 3: Attach it to this session with push access**

```
add_repo(owner="aleogr", repo="lab-shared", access="push")
```

Expected: `status: attached` and a clone path. If the call is refused, stop:
the rest of this plan writes to that repository, and there is no way around it
that does not put a credential somewhere it should not be.

- [ ] **Step 4: Write README.md**

```markdown
# lab-shared

The Cloud SQL instance the lab databases of `aleogr/marketplace` and
`codeschool-ing/schooling` share, and nothing else.

## What this owns

The project `aleogr-lab-shared-<4>`, the instance `lab-postgres`, and the IAM
that lets each tenant declare its own database inside it.

## What this does not own

**Any tenant's database, roles or grants.** Those are declared by the tenant,
in the tenant's own repository, against this project — `google_sql_database`
and `google_sql_user` both take `project` and `instance`. The boundary is
deliberate: whoever looks after the house is the house; whoever looks after a
room is whoever lives in it. A tenant cannot change the instance, and this
repository cannot change a tenant's data.

## The rule that decides what may live here

**Consolidate by environment, never across environments.** Every database in
this instance holds a lab. The day one of them becomes production it leaves.

See `docs/superpowers/specs/2026-09-20-shared-database-instance-design.md` in
`aleogr/marketplace` for why, including what this arrangement does *not*
protect against.
```

- [ ] **Step 5: Write CLAUDE.md**

```markdown
# Working agreements

- Talk to the owner in **Portuguese**. Everything versioned is in **English**.
- This repository is **public**. No secret may ever be committed.
- **Terraform is only ever applied by CI.** A session never runs `apply`.
- Claude Code opens the pull request and **never merges**. The owner merges.
- This repository owns the instance. It never declares a tenant's database,
  role or grant — see README.md.
- Before pushing: `terraform fmt -check`, `terraform init -backend=false`,
  `terraform validate`.
```

- [ ] **Step 6: Commit and push**

```sh
cd /home/user/lab-shared
git checkout -b claude/shared-instance
git add README.md CLAUDE.md
git commit -m "The repository that owns the shared lab instance, and only that"
git push -u origin claude/shared-instance
```

---

### Task 3: The deploy identity, so CI can apply

**Files:**
- Create: `infra/terraform/versions.tf`, `providers.tf`, `backend.tf`, `variables.tf`, `services.tf`, `deployer.tf`, `lab/backend.hcl`, `lab/lab.tfvars` in `aleogr/lab-shared`

**Interfaces:**
- Consumes: the project id and bucket from Task 1.
- Produces: a service account `deployer@<shared project>.iam.gserviceaccount.com` federated to `aleogr/lab-shared`, which Task 4's CI uses.

- [ ] **Step 1: versions.tf**

```hcl
terraform {
  required_version = ">= 1.9.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}
```

- [ ] **Step 2: providers.tf**

```hcl
# `billing_project` is stated rather than inherited. Left out, the provider
# takes whatever `gcloud config set project` last wrote — which is not a
# default but a leftover, and it changes when a Cloud Shell session restarts.
# Every call would then be charged to another project's quota and refused, with
# an error naming the wrong project in its advice.
provider "google" {
  project               = var.project_id
  region                = var.region
  billing_project       = var.project_id
  user_project_override = true
}
```

- [ ] **Step 3: backend.tf**

```hcl
# The bucket cannot be created by the configuration that needs it, so it is
# made by hand and recorded in docs/infrastructure.md of aleogr/marketplace.
# Its name reaches CI as the repository variable TF_STATE_BUCKET; the prefix
# comes from lab/backend.hcl. Both arrive through -backend-config.
terraform {
  backend "gcs" {}
}
```

- [ ] **Step 4: variables.tf**

```hcl
variable "project_id" {
  description = "The shared project. Permanent, and never reused once created."
  type        = string
}

variable "region" {
  description = "Where the instance runs. The same region as every tenant that uses it, because a cross-region socket is latency nobody asked for."
  type        = string
  default     = "us-central1"
}

variable "database_tier" {
  description = "Cloud SQL machine type. Shared-core tiers require edition ENTERPRISE, which is stated out loud in instance.tf."
  type        = string
  default     = "db-f1-micro"
}

variable "database_disk_size" {
  description = "Size in GB of the disk, which is also its floor: Cloud SQL grows a disk and never shrinks one."
  type        = number
  default     = 10
}

variable "database_disk_limit" {
  description = "Ceiling for automatic growth. Growth is one-way, so an unbounded limit turns one runaway write into a permanent bill."
  type        = number
  default     = 50
}

variable "tenants" {
  description = "The deployer service accounts allowed to declare a database inside this instance. One entry per tenant repository."
  type        = map(string)
}
```

- [ ] **Step 5: services.tf**

```hcl
resource "google_project_service" "enabled" {
  for_each = toset([
    "sqladmin.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "cloudscheduler.googleapis.com",
  ])

  service = each.value

  # Turning an API off does not remove what it created; it breaks it. A
  # `terraform destroy` that disabled sqladmin would leave an instance nobody
  # can read.
  disable_on_destroy = false
}
```

- [ ] **Step 6: deployer.tf**

```hcl
# The identity CI acts as, and it holds no key. GitHub mints a token, the
# federation exchanges it for a short-lived credential, and nothing long-lived
# exists to leak.
resource "google_service_account" "deployer" {
  account_id   = "deployer"
  display_name = "GitHub Actions, applying this configuration"
  description  = "Federated to aleogr/lab-shared. Owns the instance, and no tenant's data."
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github"
  display_name              = "GitHub Actions"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }

  # WITHOUT THIS, ANY REPOSITORY ON GITHUB CAN ASK. The condition is what makes
  # the federation an authorisation rather than an introduction.
  attribute_condition = "assertion.repository == 'aleogr/lab-shared'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "deployer_federation" {
  service_account_id = google_service_account.deployer.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${data.google_project.current.number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.github.workload_identity_pool_id}/attribute.repository/aleogr/lab-shared"
}

resource "google_project_iam_member" "deployer_sql" {
  project = var.project_id
  role    = "roles/cloudsql.admin"
  member  = "serviceAccount:${google_service_account.deployer.email}"
}

resource "google_project_iam_member" "deployer_iam" {
  project = var.project_id
  role    = "roles/resourcemanager.projectIamAdmin"
  member  = "serviceAccount:${google_service_account.deployer.email}"
}

data "google_project" "current" {
  project_id = var.project_id
}
```

- [ ] **Step 7: lab/backend.hcl and lab/lab.tfvars**

```hcl
# lab/backend.hcl — the bucket comes from TF_STATE_BUCKET, see backend.tf
prefix = "lab"
```

```hcl
# lab/lab.tfvars — project_id arrives from the repository variable GCP_PROJECT_ID
region = "us-central1"

# Filled in by Task 5, when the marketplace deployer's address is known.
tenants = {}
```

- [ ] **Step 8: Validate locally**

```sh
cd /home/user/lab-shared
terraform -chdir=infra/terraform fmt -check
terraform -chdir=infra/terraform init -backend=false
terraform -chdir=infra/terraform validate
```

Expected: `Success! The configuration is valid.` and no output from `fmt`.

- [ ] **Step 9: Commit**

```sh
git add infra/terraform
git commit -m "The deploy identity, federated to this repository and nothing else"
git push
```

- [ ] **Step 10: Owner bootstraps the first apply**

The federation cannot apply itself: the first `apply` must come from an
identity that already exists. Hand the owner this, and wait:

```sh
terraform -chdir=infra/terraform init \
  -backend-config="bucket=$BUCKET" -backend-config="prefix=lab"
terraform -chdir=infra/terraform plan \
  -var="project_id=$PROJECT" -var-file=lab/lab.tfvars     # read it
terraform -chdir=infra/terraform apply \
  -var="project_id=$PROJECT" -var-file=lab/lab.tfvars
```

Expected: the plan creates the service account, the pool, the provider and the
IAM bindings — and **no instance**, which comes in Task 4. This is the one
apply that does not come from CI, and the reason is written above.

---

### Task 4: The instance, and a check that proves its settings

**Files:**
- Create: `infra/terraform/instance.tf`, `infra/terraform/outputs.tf`, `tools/check-instance.sh`, `.github/workflows/ci.yml` in `aleogr/lab-shared`

**Interfaces:**
- Consumes: the deployer from Task 3.
- Produces: `lab-postgres`, whose connection name is `<project>:us-central1:lab-postgres`, read by Task 5.

- [ ] **Step 1: Write the check first, and watch it fail**

`tools/check-instance.sh`. It is the test: it reads the live instance and
asserts every setting the design commits to. Written before the instance
exists, so its first run fails for the right reason.

```sh
#!/usr/bin/env bash
# What the design promises about this instance, written as assertions.
#
# It reads the LIVE instance rather than the configuration, because a
# configuration that says the right thing and was never applied looks exactly
# like one that was. Run after every apply that touches instance.tf.
set -euo pipefail

project="${1:?usage: check-instance.sh <project> <instance>}"
instance="${2:?usage: check-instance.sh <project> <instance>}"

actual="$(gcloud sql instances describe "$instance" --project="$project" --format=json)"
failures=0

check() {
  local path="$1" want="$2"
  local got
  got="$(printf '%s' "$actual" | python3 -c "
import json,sys
o=json.load(sys.stdin)
for k in '$path'.split('.'):
    o = o.get(k) if isinstance(o, dict) else None
print(json.dumps(o))
")"
  if [ "$got" != "$want" ]; then
    printf '%-60s %s, want %s\n' "$path" "$got" "$want"
    failures=$((failures + 1))
  fi
}

check databaseVersion '"POSTGRES_16"'
check settings.edition '"ENTERPRISE"'
check settings.tier '"db-f1-micro"'
check settings.availabilityType '"ZONAL"'
check settings.dataDiskType '"PD_SSD"'
check settings.dataDiskSizeGb '"10"'
check settings.storageAutoResize 'true'
check settings.storageAutoResizeLimit '"50"'
check settings.deletionProtectionEnabled 'true'
check settings.backupConfiguration.enabled 'true'
check settings.backupConfiguration.startTime '"12:00"'
check settings.backupConfiguration.pointInTimeRecoveryEnabled 'true'
check settings.backupConfiguration.transactionLogRetentionDays '7'
check settings.ipConfiguration.sslMode '"ENCRYPTED_ONLY"'

# The flag is a list, not a field, so it is read on its own.
if ! printf '%s' "$actual" | python3 -c "
import json,sys
flags = json.load(sys.stdin)['settings'].get('databaseFlags', [])
sys.exit(0 if {'name':'cloudsql.iam_authentication','value':'on'} in flags else 1)
"; then
  echo 'cloudsql.iam_authentication is not on'
  failures=$((failures + 1))
fi

if [ "$failures" -ne 0 ]; then
  echo "$failures setting(s) do not match the design"
  exit 1
fi
echo "the instance matches the design"
```

- [ ] **Step 2: Run it and verify it fails**

```sh
chmod +x tools/check-instance.sh
./tools/check-instance.sh "$PROJECT" lab-postgres
```

Expected: FAIL, `ERROR: (gcloud.sql.instances.describe) ... was not found`. The
instance does not exist yet; that is the point.

- [ ] **Step 3: instance.tf**

```hcl
/* The instance both labs share, and the only thing this repository owns.

   IT DOES NOT SCALE TO ZERO, which is why it sleeps instead. Cloud Run costs
   nothing while nobody is reading; this is charged by the hour whether or not
   anybody is, and it is the whole standing cost of both projects. The schedule
   that stops it on weeknights is Plan 2.

   THE EDITION IS SAID OUT LOUD. Left out, the API picks ENTERPRISE_PLUS, where
   shared-core tiers do not exist at all, and refuses `db-f1-micro` with a
   message suggesting a machine several times this bill. The tier and the
   edition are one decision, and the API will make the half nobody wrote down.

   NOTHING HERE NAMES A TENANT. A database, a role or a grant in this file
   would make this repository the owner of somebody else's data. They are
   declared by their tenants, against this instance. */
resource "google_sql_database_instance" "shared" {
  name             = "lab-postgres"
  database_version = "POSTGRES_16"
  region           = var.region

  deletion_protection = true

  settings {
    edition           = "ENTERPRISE"
    tier              = var.database_tier
    availability_type = "ZONAL"

    deletion_protection_enabled = true

    disk_type             = "PD_SSD"
    disk_size             = var.database_disk_size
    disk_autoresize       = true
    disk_autoresize_limit = var.database_disk_limit

    backup_configuration {
      enabled = true

      # NOON UTC, AND THE HOUR IS THE WHOLE POINT. A stopped instance runs no
      # automated backup, and this instance is stopped from 22:00 to 07:45
      # local on weeknights. A backup window in the small hours would mean both
      # projects quietly stopped having daily backups — no error, nothing to
      # notice, just an absence.
      start_time = "12:00"

      point_in_time_recovery_enabled = true
      transaction_log_retention_days = 7

      backup_retention_settings {
        retained_backups = 7
        retention_unit   = "COUNT"
      }
    }

    ip_configuration {
      # A public address with NO authorized networks, which is not the
      # contradiction it reads as: with an empty list nothing on the internet
      # can open a connection. The way in is the Cloud SQL connector, which
      # authenticates with IAM and encrypts per connection.
      ipv4_enabled = true
      ssl_mode     = "ENCRYPTED_ONLY"
    }

    database_flags {
      # The marketplace service authenticates with a token instead of a
      # password. Password users are unaffected, so `schooling` keeps working
      # exactly as it does today.
      name  = "cloudsql.iam_authentication"
      value = "on"
    }
  }

  depends_on = [google_project_service.enabled]
}
```

- [ ] **Step 4: outputs.tf**

```hcl
output "connection_name" {
  description = "How a tenant addresses this instance: <project>:<region>:<instance>. It is what Cloud Run mounts and what the connector dials."
  value       = google_sql_database_instance.shared.connection_name
}

output "instance_name" {
  description = "The instance's own name, for a tenant declaring a database inside it."
  value       = google_sql_database_instance.shared.name
}
```

- [ ] **Step 5: .github/workflows/ci.yml**

```yaml
name: CI

# Plan on a pull request so the change can be read; apply only from main, so
# nothing reaches the project that was not merged.
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  contents: read
  id-token: write
  pull-requests: write

jobs:
  terraform:
    name: Format, validate, plan and apply
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v7

      - uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: 1.9.8

      - uses: google-github-actions/auth@v2
        with:
          project_id: ${{ vars.GCP_PROJECT_ID }}
          workload_identity_provider: ${{ vars.WORKLOAD_IDENTITY_PROVIDER }}
          service_account: deployer@${{ vars.GCP_PROJECT_ID }}.iam.gserviceaccount.com

      - uses: google-github-actions/setup-gcloud@v2

      - name: Format
        run: terraform -chdir=infra/terraform fmt -check -recursive

      - name: Init
        run: |
          terraform -chdir=infra/terraform init \
            -backend-config="bucket=${{ vars.TF_STATE_BUCKET }}" \
            -backend-config=lab/backend.hcl

      - name: Validate
        run: terraform -chdir=infra/terraform validate

      - name: Plan
        run: |
          terraform -chdir=infra/terraform plan \
            -var="project_id=${{ vars.GCP_PROJECT_ID }}" \
            -var-file=lab/lab.tfvars

      - name: Apply
        if: github.ref == 'refs/heads/main' && github.event_name == 'push'
        run: |
          terraform -chdir=infra/terraform apply -auto-approve \
            -var="project_id=${{ vars.GCP_PROJECT_ID }}" \
            -var-file=lab/lab.tfvars

      # THE CHECK RUNS AFTER THE APPLY AND NOT INSTEAD OF IT. A configuration
      # that says the right thing and was never applied reads exactly like one
      # that was; this asks the live instance.
      - name: The instance matches the design
        if: github.ref == 'refs/heads/main' && github.event_name == 'push'
        run: ./tools/check-instance.sh "${{ vars.GCP_PROJECT_ID }}" lab-postgres
```

- [ ] **Step 6: Owner sets the three repository variables**

```sh
# Settings → Secrets and variables → Actions → Variables, in aleogr/lab-shared:
#   GCP_PROJECT_ID              the project id from Task 1
#   TF_STATE_BUCKET             the bucket from Task 1
#   WORKLOAD_IDENTITY_PROVIDER  printed by this:
gcloud iam workload-identity-pools providers describe github \
  --project="$PROJECT" --location=global --workload-identity-pool=github \
  --format='value(name)'
```

Expected: a value shaped
`projects/<number>/locations/global/workloadIdentityPools/github/providers/github`.
These are **variables, not secrets** — none of the three is confidential, and
putting a non-secret in the secret store only makes logs harder to read.

- [ ] **Step 7: Validate, commit, open the pull request**

```sh
terraform -chdir=infra/terraform fmt -check -recursive
terraform -chdir=infra/terraform init -backend=false
terraform -chdir=infra/terraform validate
git add infra/terraform tools .github
git commit -m "The shared instance, and a check that reads the live one"
git push
```

Then open the pull request. **The owner merges**; CI applies and runs the
check.

- [ ] **Step 8: Verify the check now passes**

After the merge, read the `The instance matches the design` step in the run.

Expected: `the instance matches the design`. Any line printed before that names
a setting and the value it actually has — fix the configuration, never the
check.

---

### Task 5: The marketplace database, declared from this repository

**Files:**
- Modify: `infra/terraform/variables.tf`, `infra/terraform/locals.tf`, `infra/terraform/cloud_sql.tf`, `infra/terraform/lab/lab.tfvars` in `aleogr/marketplace`
- Modify: `infra/terraform/lab/lab.tfvars` in `aleogr/lab-shared` (the `tenants` map)

**Interfaces:**
- Consumes: `connection_name` and `instance_name` from Task 4.
- Produces: `local.database_instance`, the single place the connection name is assembled, read by Task 6.

- [ ] **Step 1: Grant this repository's deployer the right to declare a database**

In `aleogr/lab-shared`, `infra/terraform/tenants.tf`:

```hcl
# What a tenant may do inside this instance: declare its own database and its
# own users, and nothing else. `cloudsql.editor` cannot delete the instance and
# cannot change its settings, which is the boundary this repository exists to
# hold.
resource "google_project_iam_member" "tenants" {
  for_each = var.tenants

  project = var.project_id
  role    = "roles/cloudsql.editor"
  member  = "serviceAccount:${each.value}"
}
```

And in `lab/lab.tfvars`, replacing the empty map:

```hcl
tenants = {
  marketplace = "deployer@aleogr-marketplace-lab-a4j5.iam.gserviceaccount.com"
}
```

Commit, pull request, owner merges, CI applies.

- [ ] **Step 2: Add the two variables here**

`infra/terraform/variables.tf`:

```hcl
variable "shared_project_id" {
  description = "The project holding the shared lab database instance. This configuration declares its own database inside it and has no rights over the instance itself (aleogr/lab-shared)."
  type        = string
}

variable "shared_instance" {
  description = "The shared instance's name. The connection name is assembled from this, the project and the region, in locals.tf."
  type        = string
  default     = "lab-postgres"
}
```

- [ ] **Step 3: Assemble the connection name in one place**

`infra/terraform/locals.tf`, appended:

```hcl
locals {
  # ONE PLACE, because three files read it and a fourth will. A connection name
  # is `<project>:<region>:<instance>` and it is assembled rather than typed,
  # so a change to any of the three reaches every reader at once.
  database_instance = "${var.shared_project_id}:${var.region}:${var.shared_instance}"
}
```

- [ ] **Step 4: Point the database, users and grants at the shared project**

In `cloud_sql.tf`, delete the `google_sql_database_instance "main"` resource
entirely, and give every resource that referenced it the shared project. The
`google_sql_database`, `google_sql_user` and Secret Manager resources stay —
they are this tenant's, not the instance's.

```hcl
resource "google_sql_database" "marketplace" {
  project  = var.shared_project_id
  name     = "marketplace"
  instance = var.shared_instance

  # ABANDON, NOT DELETE, and the default is DELETE. The instance's protection
  # does not reach here: dropping a database is not deleting an instance, and
  # a rename Terraform reads as a replacement would drop it with every row in
  # it while the protected instance stood.
  deletion_policy = "ABANDON"
}

resource "google_sql_user" "service" {
  project  = var.shared_project_id
  name     = trimsuffix(google_service_account.service.email, ".gserviceaccount.com")
  instance = var.shared_instance
  type     = "CLOUD_IAM_SERVICE_ACCOUNT"
}

resource "google_sql_user" "migrator" {
  project  = var.shared_project_id
  name     = "marketplace_migrator"
  instance = var.shared_instance
  password = random_password.migrator.result
}
```

The rename from `migrator` to `marketplace_migrator` is the spec's naming rule:
a role belongs to the cluster, so a role named for its function rather than its
tenant is a trap the day a third tenant arrives.

- [ ] **Step 5: Replace every reference to the removed resource**

Three files read `google_sql_database_instance.main.connection_name`. Each
becomes `local.database_instance`:

```
infra/terraform/cloud_run.tf:115
infra/terraform/migrate_job.tf:55
infra/terraform/outputs.tf:28
```

- [ ] **Step 6: Add the shared project to the environment's values**

`infra/terraform/lab/lab.tfvars`:

```hcl
# The lab databases of this project and of `schooling` share one instance,
# in a project neither of them owns (aleogr/lab-shared). This configuration
# declares its own database inside it and cannot change the instance.
shared_project_id = "aleogr-lab-shared-<4>"
```

`<4>` is the suffix Task 1 generated. Substitute the real value — this is the
one line in the plan that cannot be pasted as written, because the id does not
exist until Task 1 has run.

- [ ] **Step 7: Validate**

```sh
make tf
```

Expected: `Success! The configuration is valid.`

- [ ] **Step 8: Commit**

```sh
git add infra/terraform
git commit -m "Declare the marketplace database inside the shared instance"
```

---

### Task 6: Both deployment surfaces name the same instance, and a test says so

**Files:**
- Modify: `cmd/marketplace/deployment_test.go` in `aleogr/marketplace`

**Interfaces:**
- Consumes: `local.database_instance` from Task 5.

- [ ] **Step 1: Write the failing test**

Pointing the service at one instance and the migration job at another is the
same class of defect as `AUDIT_KEY` missing from one surface: both files are
green on their own, and the deployment is wrong. Append to
`cmd/marketplace/deployment_test.go`:

```go
// TestBothDeploymentSurfacesNameTheSameDatabase is the lesson of AUDIT_KEY
// applied to the database.
//
// The service and the migration job run the same binary against the same data.
// If one is pointed at the shared instance and the other at anything else, the
// migrations land where nobody reads them and nothing fails: two files, each
// correct on its own, and a deployment that is wrong.
//
// It checks that both read the same expression rather than that they hold the
// same literal, because the literal is assembled in locals.tf and a test
// asserting a literal would be a second copy of it.
func TestBothDeploymentSurfacesNameTheSameDatabase(t *testing.T) {
	const wanted = "local.database_instance"

	for surface, path := range surfaces {
		declared, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("cannot read the Terraform of %s: %v", surface, err)
		}

		if !strings.Contains(string(declared), wanted) {
			t.Errorf("%s does not read %s for DATABASE_INSTANCE; "+
				"a surface pointed at another instance migrates where nobody reads",
				surface, wanted)
		}
	}
}
```

- [ ] **Step 2: Run it and verify it fails**

Stash Task 5's Terraform changes first, so the test fails against the old
files rather than passing by accident:

```sh
git stash push infra/terraform
go test ./cmd/marketplace/ -run TestBothDeploymentSurfacesNameTheSameDatabase -v
```

Expected: FAIL, twice —
`the service does not read local.database_instance for DATABASE_INSTANCE`.

- [ ] **Step 3: Restore Task 5's changes**

```sh
git stash pop
```

- [ ] **Step 4: Run it and verify it passes**

```sh
go test ./cmd/marketplace/ -run TestBothDeploymentSurfacesNameTheSameDatabase -v
```

Expected: PASS.

- [ ] **Step 5: Run everything**

```sh
make check
make test
make tf
```

Expected: `0 issues`, all packages `ok`, `Success! The configuration is valid.`

- [ ] **Step 6: Commit and open the pull request**

```sh
git add cmd/marketplace/deployment_test.go
git commit -m "Refuse a deployment whose two surfaces migrate to different databases"
git push -u origin claude/funny-wright-379asb-shareddb
```

Open the pull request. **The owner merges**, and the merge is what proves the
rehearsal: the migration job builds the schema in the shared instance, seeds
the marketplaces, writes their audit records, and `make e2e-lab` runs against
the deployed service.

- [ ] **Step 7: Read the deployment, and say what it proved**

After the merge, confirm in the run:

- `Apply to the lab` succeeded — the database and the two users exist in the
  shared project.
- `Build and deploy to the lab` succeeded — the migration job started, which
  means the connector reached across projects and the IAM user could log in.
- `make e2e-lab` passed — the service reads and writes.

If any of the three fails, this is the rehearsal doing its job. Diagnose from
that job's log, not from a guess.

---

### Task 7: Isolation, the old instance stopped, and what was learned

**Files:**
- Modify: `docs/infrastructure.md` in `aleogr/marketplace`

- [ ] **Step 1: Apply the isolation grants**

Owner-executed, through the Cloud SQL Auth Proxy, once the database exists.
By default every role may connect to every database in the cluster, so without
this the `schooling` role — which arrives in Plan 2 — would reach this data.

```sql
REVOKE CONNECT ON DATABASE marketplace FROM PUBLIC;
GRANT  CONNECT ON DATABASE marketplace TO marketplace_migrator;
ALTER DATABASE marketplace CONNECTION LIMIT 10;
```

The service's own user is granted the same way. Cloud SQL derives that user's
name from the service account's address with the domain removed, which for
`marketplace-run@aleogr-marketplace-lab-a4j5.iam.gserviceaccount.com`
(`infra/terraform/cloud_run.tf:12`) is:

```sql
GRANT CONNECT ON DATABASE marketplace
  TO "marketplace-run@aleogr-marketplace-lab-a4j5.iam";
```

The quotes are required: the name holds `@` and `-`, which an unquoted
PostgreSQL identifier may not.

- [ ] **Step 2: Prove the revoke bites**

A guard nobody tried is a guard nobody has. Create a throwaway role with no
grant and watch it be refused:

```sql
CREATE ROLE connect_probe LOGIN PASSWORD 'probe-only-and-dropped-below';
```

```sh
PGPASSWORD='probe-only-and-dropped-below' psql \
  "host=127.0.0.1 port=5432 user=connect_probe dbname=marketplace" -c 'SELECT 1'
```

Expected: `FATAL:  permission denied for database "marketplace"`. If it
connects, the `REVOKE` did not take and nothing below should proceed.

Then, whatever the outcome:

```sql
DROP ROLE connect_probe;
```

The password is in this plan on purpose: it belongs to a role that exists for
one command and is dropped in the next. Nothing it could reach is reachable
without the `CONNECT` this step is proving is absent.

- [ ] **Step 3: Stop the old instance — do not delete it**

```sh
gcloud sql instances patch marketplace \
  --project=aleogr-marketplace-lab-a4j5 --activation-policy=NEVER
```

Expected: the instance's state becomes `STOPPED`. It keeps its data and its
disk, costs about R$ 10 a month, and is the rollback if anything found later
sends this back. Plan 2's cool-down deletes it.

- [ ] **Step 4: Record the decisions in docs/infrastructure.md**

The spec is the record of how the decisions were reached; `docs/infrastructure.md`
is where the ones that survived live. Add: the shared project and what owns it,
the boundary between instance and database, the naming rule, the noon backup
window and why, and the state bucket created by hand in Task 1.

- [ ] **Step 5: Commit, pull request, owner merges**

---

## What Plan 2 will need from this one

Written down at the end of Task 7, because Plan 2 is authored from it:

- whether the cross-project connector needed anything beyond `cloudsql.editor`
  and `cloudsql.client`
- the exact shape of the connection string that worked, socket and all
- how long a stopped instance took to accept connections after `ALWAYS`
- anything the rehearsal refused that the spec did not predict
