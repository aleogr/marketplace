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

## Budget and the lab's cost

A budget named `marketplace-lab`, scoped to the lab project alone, alerts by
e-mail at **R$ 200 per month**: at 50%, 90% and 100% of actual spend, and once
more when the *forecast* for the month reaches 100%. The forecast threshold is
the one that catches a runaway on the third day rather than the twentieth; the
others catch a drift.

It is **alerts only**, never the spend-limit enforcement the console offers
beside it. That option pauses services when the cap is reached, which in an
environment whose purpose is to be available for testing trades a surprise bill
for a surprise outage.

R$ 200 is roughly three times what the lab is expected to cost, which is what
keeps the 50% alert meaningful: a threshold that sits inside the normal range
sends an e-mail every month and teaches its reader to ignore it.

**The expected cost**, at the configuration below and the rate of 2026-09-18
(1 USD = R$ 5,1359):

| Item | ~US$/month |
|---|---|
| `db-f1-micro` instance (1 shared vCPU, 0.614 GB) | 8 |
| 10 GB SSD | 1.70 |
| Backups and point-in-time recovery logs | 0.50 – 4 |
| **Total** | **~10 – 14** (R$ 51 – 72) |

Cloud SQL is the only material recurring cost of the whole design; Cloud Run,
Tasks, Scheduler, Storage and Logging stay within free quotas or cents at this
volume (`docs/design.md`, section 3).

**Why the lab is configured more cheaply than the design's targets.** Design
section 3 sets a recovery point objective of 5 minutes, 30-day backup retention
and a quarterly restore test. Those are **production's** targets. The lab holds
nothing irreplaceable — it is rebuilt from this repository — so it keeps 7
backups and a 1-day point-in-time window, the shortest Cloud SQL Enterprise
allows. Point-in-time recovery stays *on* rather than off: it costs almost
nothing on an idle database, and the mechanism production will depend on should
have been exercised somewhere before production depends on it.

**The tier is `db-f1-micro`**, the cheapest Cloud SQL offers and the one Google
documents as being for test and development. Its 0.614 GB of memory is tight for
PostgreSQL, and that is an accepted risk rather than an overlooked one: the tier
is a Terraform variable, and changing it restarts the instance without touching
the data. Starting cheap is reversible; starting expensive is only reversible by
noticing.

**The budget is not in Terraform**, and that is deliberate: a budget resource
needs the billing account id, which this document does not record because the
repository is public and that id identifies a payment instrument.

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

### What the bootstrap produced, verified afterwards

`gcloud storage buckets describe gs://aleogr-marketplace-lab-a4j5-tfstate`, run
in Cloud Shell on 2026-09-18. This is the third verification item of delivery
F2 (`docs/roadmap.md`): the state bucket is the one resource whose loss is not
recoverable from this repository, so its properties are read back rather than
assumed from the block above.

| Property | Value |
|---|---|
| `versioning_enabled` | `true` |
| `uniform_bucket_level_access` | `true` |
| `public_access_prevention` | `enforced` |
| `location` | `US-CENTRAL1` |

The bucket also reports a `soft_delete_policy` of seven days, which Cloud
Storage applies by default and the block above never asked for. It is a second
recovery layer rather than a replacement for versioning, because the two answer
different questions: versioning answers "the state was overwritten with
something broken", soft delete answers "the state was deleted". Neither costs
anything measurable for a file of this size.

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
| `GCP_DEPLOYER_SA` | `deployer@aleogr-marketplace-lab-a4j5.iam.gserviceaccount.com` |

The billing account id is deliberately **not** recorded here: the repository is
public and it identifies a payment instrument.

## Terraform

Everything above exists so that this can run. The configuration lives in
`infra/terraform/` and is applied only by GitHub Actions
(`.github/workflows/terraform.yml`); nobody applies from a laptop, and a Claude
Code session has no credentials to apply with.

```
infra/terraform/
  .terraform-version   the Terraform release, read by CI and by make terraform-deps
  .terraform.lock.hcl  the provider versions and their checksums
  versions.tf          the Terraform and provider constraints
  backend.tf           backend "gcs" {}, deliberately empty
  providers.tf         project and region, no credentials
  variables.tf         project_id, region, environment
  locals.tf            the labels every resource carries
  artifact_registry.tf the container registry
  outputs.tf
  lab/
    backend.hcl        prefix = "lab"
    lab.tfvars         region and environment
```

**One root configuration, one directory of values per environment.** The
resources are described once; `lab/` holds what makes them the lab. Production
adds `prod/` next to it, and nothing else moves. That is why `backend.tf` is
empty and `project_id` has no default: the values that differ between
environments are supplied at `init` and `plan` time, from the environment's
directory and from the repository variables, never from a second copy of the
configuration.

**The state bucket's name is a repository variable, not a line in this
repository.** `terraform init` receives it as
`-backend-config="bucket=$TF_STATE_BUCKET"`, and the state prefix comes from
`lab/backend.hcl`. Two environments therefore cannot write to one state file by
forgetting to change a line.

**What the workflow does**

| Job | When | What it needs |
|---|---|---|
| Format and validate | every pull request and every push to `main` | nothing; no backend, no credentials |
| Plan the lab | pull requests from this repository | the federation; posts the plan as a single comment, rewritten on each run |
| Apply to the lab | pushes to `main` | the federation |

The plan job is skipped for a pull request from a fork and for Dependabot,
because the federation issues credentials only against a token minted for this
repository and Dependabot's token is read-only. It is for that reason not a
required check: **Format and validate** is the one that must pass.

**What Terraform does not manage.** The resources of the bootstrap block above
— the state bucket, the service account, the federation — and the enabled APIs.
Terraform cannot create the bucket that holds its own state or the identity it
authenticates with, and adopting the API enablement would mean a `destroy` could
turn the project off. They are created once, by hand, and recorded here.

## The deployment pipeline

Three workflows act on `main`, in this order, and each one answers a different
question.

| Workflow | Trigger | What it does |
|---|---|---|
| `terraform.yml` | push to `main` | applies the infrastructure |
| `deploy.yml` | `terraform.yml` finished successfully on `main` | builds the image, pushes it, **applies the migrations**, rolls the service forward, proves the deployment answers |
| `release.yml` | a `v*` tag | creates the GitHub Release |

**Why `deploy.yml` waits for `terraform.yml` instead of listening to the same
push.** On the merge that first creates the Cloud Run service there is nothing
to deploy to until Terraform has applied, and on every merge afterwards two
writers of the same service at once is a race Cloud Run answers with a
conflict. Chaining them costs a few seconds and removes both.

**Terraform owns the service's shape; the pipeline owns its image.** Terraform
declares the identity the service runs as, how it scales, every environment
variable it reads and the host it answers on, and explicitly ignores the image,
which changes on every merge. Without that split, a deployment would be an
infrastructure change and every `terraform plan` would report a difference left
by the last one. It is also why the service is created with Google's `hello`
image: Terraform cannot create a service without naming one, and on that first
merge no image of this repository exists yet.

**Nothing Terraform declares may depend on an image that is not deployed
yet.** This follows from the order above and is the one rule that makes it
safe. Terraform applies first, and the revision it creates runs whatever image
the service is already on — so a Terraform setting that only the *next* image
satisfies fails the apply, and the failed apply skips the deployment that would
have brought that image. Each half then waits for the other. It happened once,
on the merge that moved the health check off `/healthz`: the apply changed the
startup probe to the new path, the revision still carried the old image, the
probe failed, and the deployment never ran. The startup probe is a TCP check on
the container port for that reason, not for simplicity — it answers the
question that matters, whether the process is listening, without naming
anything the application might rename.

**Deployment is by digest, never by tag.** A tag is a label that can be moved
onto other bytes; a digest is the bytes. It is also what lets a release promote
the image that was already tested rather than rebuild its source
(`docs/requirements.md`, section 27). Tags in the registry are therefore
mutable and exist for people reading it.

**Migrations run before the service, never after.** A revision that met a
schema older than the code expects would fail on the first query rather than at
deployment time, and it would fail for a visitor. The migration job runs the
same image the service is about to run, so the schema a deployment applies and
the code that expects it are always from one commit; `gcloud run jobs execute
--wait` is what makes a failed migration a failed deployment rather than a
started one.

### The accounts, and what each may do

| Account | Used by | May |
|---|---|---|
| `terraform@…` | `terraform.yml` | create and destroy the project's infrastructure |
| `deployer@…` | `deploy.yml` | push to the `containers` registry, update the `marketplace` service and the migration job, and act as their runtime accounts |
| `marketplace-run@…` | the running service | write logs; connect to the database as itself, with IAM |
| `marketplace-migrate@…` | the migration job | write logs; connect to the database as the migration user, and read that user's password |

They are separate because they run on different triggers and fail differently:
a mistake in a deployment must not be able to delete a database. The deploy
account's roles are granted on the registry and on the service rather than on
the project, so a second registry or a second service added later is not
writable by accident. Neither account holds a key; both federate from GitHub
through the pool the bootstrap block created.

`deployer@…` is created by Terraform, but its name is predictable, which is why
the repository variable above can be set before the first apply. It has to be:
the first deployment runs minutes after that apply, and a workflow cannot read
a variable that does not exist yet.

### Domain verification, performed by hand

Cloud Run refuses to map a domain to a service until the account that creates
the mapping is a **verified owner of the base domain** in Google Search Console.
The mapping is created by `terraform@…` during the apply, so it is that account
— not the deploy account, and not the owner's own Google account — that must be
on the domain's verified-owner list.

Performed once, in [Search Console](https://search.google.com/search-console),
on the `aleogr.dev` property: **Settings → Users and permissions → Add user**,
with `terraform@aleogr-marketplace-lab-a4j5.iam.gserviceaccount.com` and the
permission **Owner**.

The "Ownership verification" screen is not where this is done, despite its
name: it lists the verification methods used, and offers no way to add an
account. A subdomain property does not help either — Search Console will hold
one for `lab.aleogr.dev`, but Cloud Run asks for ownership of the registrable
domain when it maps any host beneath it.

Without this, the apply fails with *"Caller is not authorized to administer the
domain"*, which names neither the account nor the console it is missing from.

**Why a domain mapped by hand needs none of this.** Mapping a domain from the
console or from `gcloud` runs as the person doing it, who is already an owner of
the property. Terraform runs as a service account, and Google has no reason to
believe that account is the same person. Every project that maps a host under
this domain from CI therefore adds its own service account here, and production
will add a third. Ownership is per account, so a new one disturbs none of the
others.

The grant is over the Search Console property, not over DNS: the account can
map hosts under `aleogr.dev` to Cloud Run services in its own project and
administer the property, and cannot touch the zone in Cloudflare. It is
revocable from the same screen. The spike that re-examines domain mapping before
production (`docs/design.md`, section 7) is also what would end the need for it:
neither a load balancer nor Cloudflare as a proxy asks for ownership here.

### The host

`marketplace.lab.aleogr.dev` is a CNAME to `ghs.googlehosted.com` in Cloudflare,
in **DNS only** mode (`docs/requirements.md`, section 7). The mode is not a
preference: with Cloudflare proxying, the host resolves to Cloudflare and Google
can never validate the domain to issue a certificate. The public resolution is
the check —

```
$ getent hosts marketplace.lab.aleogr.dev
2607:f8b0:4001:c64::79   ghs.googlehosted.com   marketplace.lab.aleogr.dev
```

— an address in Google's range rather than in Cloudflare's. Certificate
issuance usually takes about fifteen minutes and can take up to 24 hours, so
the pipeline verifies a deployment against the service's `run.app` URL and
never against this one.

Domain mapping is a preview feature Google does not recommend for production,
and the spike that decides the production approach is scheduled before launch
(`docs/design.md`, section 7). Nothing depends on its outcome: the service
resolves the marketplace from the host it receives, whatever put it there.

## The database

One Cloud SQL instance, `marketplace`, PostgreSQL 16, described in
`infra/terraform/cloud_sql.tf`. What it costs and why it is configured the way
it is are above; what reaches it is here.

**Two identities, and only one password.** The service authenticates with IAM:
the connector exchanges its service account's token for the login, so a running
revision holds no credential at all. The migration job cannot do the same,
because it is the thing that bootstraps the privileges — Cloud SQL creates an
IAM database user with no rights on anything, and granting rights needs a
connection that already has them. That first connection is a built-in
`migrator` user whose password Terraform generates and writes to Secret
Manager, where only the job's account may read it. The value is never in this
repository, never in a workflow's environment and never in a log. It is also in
the Terraform state, which lives in the private, versioned bucket above.

**The migrations grant, and keep granting.** After applying the schema the job
grants the service's user what the schema now holds, and sets default
privileges so that a table created by a later migration carries the same rights
without that migration having to remember. A delivery that forgot would
otherwise fail in the lab and not in a test.

**A public address with no authorized network** is not the contradiction it
reads as: with an empty authorization list nothing on the internet can open a
connection, and the only route in is the connector. Private IP only would
require a VPC with private services access, and Cloud SQL refuses an instance
with neither — this design has no network of its own to maintain.

**Reading the database by hand**, from Cloud Shell:

```bash
gcloud sql connect marketplace --user=postgres --database=marketplace \
  --project=aleogr-marketplace-lab-a4j5
```

It authorizes the client address for a few minutes and then removes it again.

## Branch protection

The `main` branch is covered by the `protect-main` ruleset: a pull request is
required, the required status checks must pass, branches must be up to date
before merging, deletions are restricted and force pushes are blocked.

| Required check | Workflow |
|---|---|
| `Build, lint and test` | `ci.yml` |
| `End-to-end tests` | `ci.yml` |
| `Secret detection` | `ci.yml` |
| `Image build and scan` | `ci.yml` |
| `Integration tests` | `ci.yml` |
| `Format and validate` | `terraform.yml` |

A check is named after its **job**, not after its workflow. The pull request
page shows `Terraform / Format and validate`, which is the workflow and the job
joined for display; the ruleset matches the check run's own name, which is
`Format and validate`.

**`Plan the lab` is deliberately not in the list.** It does not run on every
pull request — it is skipped for a pull request from a fork and for Dependabot,
because the federation issues credentials only against a token minted for this
repository and Dependabot's token is read-only. A gate has to mean the same
thing on every pull request to be a gate, and that one cannot. `Format and
validate` needs no credential, runs everywhere, and answers the question that
matters before a merge: whether `infra/terraform/` is well-formed.

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
