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

**The instance is not this project's.** The lab's PostgreSQL lives in
`aleogr-lab-shared-dacd`, a project whose only job is to hold one Cloud SQL
instance for every laboratory that needs one, declared in `aleogr/lab`.
This project declares the *database* and its *users* inside that instance
(`infra/terraform/cloud_sql.tf`) and owns nothing about the instance itself.

| | |
|---|---|
| Shared project | `aleogr-lab-shared-dacd` |
| Instance | `lab-postgres`, PostgreSQL 16, `db-f1-micro`, `us-central1` |
| Connection name | `aleogr-lab-shared-dacd:us-central1:lab-postgres` |
| Database | `marketplace` |
| Users | `marketplace_migrator` (password) and `marketplace-run@aleogr-marketplace-lab-a4j5.iam` (IAM) |

**Why one instance for several laboratories.** A `db-f1-micro` costs about
US$ 10 a month whether it holds one database or three, and one instance per
project was the largest recurring line on the bill. The rule that keeps this
from becoming a trap is written in `aleogr/lab`: **consolidate by
environment, never across environments.** Every database in that instance holds
a laboratory. The day one of them becomes production it leaves.

**Naming.** The instance is named for what it is, `lab-postgres`, and not for
any project that uses it: a shared resource named after its first tenant reads
as that tenant's property. The databases are named for their projects, so a
`\l` in `psql` says who owns what.

**The boundary between the instance and the database.** The shared repository
grants this project's Terraform identity a custom role, `sqlTenant`, that may
create and alter its own database and users and nothing else — no
`cloudsql.instances.delete`, no `cloudsql.instances.update`. A tenant cannot
resize the disk, change the backup window or stop the instance it shares.
**That role is not a wall between tenants.** Cloud SQL grants IAM per project
and not per database, so a tenant that may delete its own users may delete
another tenant's. What protects one laboratory's data from another's is the
database-level grant below, not IAM.

**Backups run at 12:00 UTC**, which is 09:00 in Paraná, and not in the small
hours where a backup window normally goes. The reason is that **a stopped
instance runs no automated backup.** The shared instance is to sleep on four
weeknights — Monday through Thursday, 22:00 to 07:45 local, which is Plan 2 and
not in effect yet — and the windows these two projects used before
consolidating, 03:00 and 04:00 local, fall inside that sleep. Left there, both
would simply stop having daily backups, silently, with no error to notice.

The same fact bounds point-in-time recovery: there is no transaction log for
the hours an instance was stopped, so a restore has to target a moment it was
awake. Point-in-time recovery is on, with seven days of transaction logs and
seven retained backups.

### Isolation, applied by hand once

PostgreSQL grants `CONNECT` on every database to `PUBLIC` by default, so a role
created for another tenant would reach this data simply by existing. Applied
through the Cloud SQL Auth Proxy as `marketplace_migrator`, which is a member
of `cloudsqlsuperuser` and therefore of the role that owns the database:

```sql
GRANT  CONNECT ON DATABASE marketplace TO marketplace_migrator;
GRANT  CONNECT ON DATABASE marketplace TO "marketplace-run@aleogr-marketplace-lab-a4j5.iam";
REVOKE CONNECT ON DATABASE marketplace FROM PUBLIC;
ALTER  DATABASE marketplace CONNECTION LIMIT 14;
```

**The grants come before the revoke on purpose.** `marketplace_migrator` held
no `CONNECT` of its own and reached the database through `PUBLIC`, so revoking
first would have locked the migration job out of the database it maintains. The
service's user already had an explicit grant, which Cloud SQL adds when it
creates an IAM user; the line is kept anyway, because a privilege that holds
only through a provider's implicit behaviour is one nobody will find when it
stops being true. The double quotes are required: the name holds `@` and `-`,
which an unquoted PostgreSQL identifier may not.

**Why the limit is 14.** It is the peak, read from the code rather than
estimated:

| | connections |
|---|---|
| service, 4 per process (`internal/platform/db/db.go`) × 2 instances (`max_instance_count`) | 8 |
| migration job, same pool | 4 |
| **peak, while a deployment migrates against the revision still serving** | **12** |

The instance allows 25 in total, of which PostgreSQL reserves three for
superusers. A ceiling of 10 — the number this was planned with, before the
pools were read — would have had the isolation refuse the deployments it exists
to protect.

**It is a ceiling, not a reservation.** It stops one tenant's connection leak
from taking the instance; it does not promise the other tenant a share. When a
second laboratory moves in, its own pool has to be measured and both ceilings
reconsidered against the 22 that are actually available.

**The revoke was proved, not assumed.** A throwaway login role with no grant,
created and dropped in the same command, was refused:

```
psql: error: ... FATAL:  permission denied for database "marketplace"
DETAIL:  User does not have CONNECT privilege.
```

A guard nobody tried is a guard nobody has.

### The identities

**Two of them, and only one password.** The service authenticates with IAM: the
connector exchanges its service account's token for the login, so a running
revision holds no credential at all. The migration job cannot do the same,
because it is the thing that bootstraps the privileges — Cloud SQL creates an
IAM database user with no rights on anything, and granting rights needs a
connection that already has them. That first connection is
`marketplace_migrator`, whose password Terraform generates and writes to Secret
Manager, where only the job's account may read it. The value is never in this
repository, never in a workflow's environment and never in a log. It is also in
the Terraform state, which lives in the private, versioned bucket above.

**The user is named for its tenant.** `marketplace_migrator`, not `migrator`:
roles are cluster-wide objects, so on a shared instance a generic name is a
collision waiting for the second tenant.

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

### Reading the database by hand

From Cloud Shell:

```bash
cloud-sql-proxy aleogr-lab-shared-dacd:us-central1:lab-postgres --port 5433 \
  > /tmp/proxy.log 2>&1 &
PROXY=$!
sleep 6

PGPASSWORD="$(gcloud secrets versions access latest \
  --secret=migrator-password --project=aleogr-marketplace-lab-a4j5)" \
  psql -h 127.0.0.1 -p 5433 -U marketplace_migrator -d marketplace \
       -c "SELECT version_id, tstamp FROM goose_db_version ORDER BY version_id;"

kill "$PROXY"
```

**The secret is in this project and the instance is in the shared one.** That
is the one thing in the command that catches a reader out.

**Neither as `postgres` nor through `gcloud sql connect`**, and each for a
reason that only shows up when you try it. There is no password for `postgres`:
Terraform creates two users and neither is it — the service's, which
authenticates with IAM and has no password at all, and `marketplace_migrator`,
whose password is in Secret Manager. `gcloud sql connect` does not pass
`PGPASSWORD` through to the `psql` it launches, so it prompts with whatever the
environment holds; running the proxy and `psql` as two steps is what lets the
password reach the program that asks for it. And `gcloud sql connect` adds the
caller's address to the instance's authorized networks — on a shared instance,
one tenant changing a setting for everybody.

### The instance this replaced

`marketplace`, in this project, was **stopped and not deleted** on 2026-09-20,
with `--activation-policy=NEVER`. It keeps its data and its disk for about
R$ 10 a month and is the way back if something found later sends this decision
back.

A stopped instance runs no automated backup, so it is a short-lived rollback
and not an archive; the backups it already had are retained for as long as it
exists. It is deleted after a week of quarantine.

### The shared project's own bootstrap

`aleogr/lab` has the same class of exception this document opens with —
things that exist because somebody typed a command, because Terraform cannot
create the ground it stands on. They are recorded in **that repository's
README**, not copied here: the project and its state bucket, the first
`terraform apply` run by the owner (a federation cannot apply itself, so the
identity that applies it has to exist first), and the grants the deploy identity
needed before it could read what it manages.

A copy in two places drifts, and the copy nobody edits is the one somebody
reads.

## Marketplaces and their hosts

A marketplace is declared once, in the environment's own `.tfvars`, and that
one declaration is used twice: Terraform maps a domain per host, and the
migration job seeds the same list into the database.

```hcl
marketplaces = [
  {
    slug    = "marketplace1"
    hosts   = ["marketplace1.marketplace.lab.aleogr.dev"]
    # ... market, revenue model, languages, contact flags
  }
]
```

**Why not a migration.** Migrations are versioned and every environment runs
all of them, so a host written into one is created everywhere — production
would hold the lab's addresses. The market stays in a migration, because Brazil
is Brazil everywhere; the hosts do not, because they are the environment's.

**Why the declaration is authoritative.** Seeding removes a host the
environment has stopped declaring. A host left behind would keep resolving to a
marketplace nobody is pointing DNS at, which is a working answer to a question
nobody asked any more.

**The DNS record is still by hand**, in Cloudflare, as a CNAME to
`ghs.googlehosted.com` in **DNS only** mode — the same shape as the platform's
own host above. Domain mapping supports no wildcard, so every marketplace costs
one record, one mapping and one certificate, and the certificate is why
creating a marketplace is not instant (`docs/requirements.md`, section 7).

**Adding a host therefore takes two deployments.** The deployment's end-to-end
step reaches every host the environment declares, so the first deployment after
a host is mapped fails while Cloud Run is still issuing that host's
certificate — usually about fifteen minutes, sometimes a day. Re-run the
deployment once the certificate exists. That failure is the price of checking
the hosts on every other deployment, which is what the delivery is about.

**Which marketplace serves a request is decided inside the process**, from the
host it receives. Every mapping points at the one Cloud Run service; there is
no routing layer and nothing to keep in step with the database.

**A host with no mapping never reaches the process at all.** Google's front end
routes by the `Host` header, and one it has no mapping for is refused with
Google's own 404 before the request arrives. The platform's own page for a host
that belongs to no marketplace is therefore what an address the edge *does*
route answers with — the service's `run.app` address, which no marketplace
claims. That is where every deployment proves it (`e2e/test_hosts.py`).

## Which entry of `X-Forwarded-For` is the client

Rate limiting keys on the client's address, so reading the wrong entry of that
header is not a detail: read one the client wrote, and a client picks a fresh
address per request and is never limited.

Google's front end **appends** to the header rather than replacing it, so the
entries to the left of what it added come from the request as it arrived. The
address is therefore counted from the right, `TRUSTED_PROXY_HOPS` entries back
(default 2, for the client address and the front end's own).

**No Cloud Run document states how many entries it adds** — it is not in the
container contract — so the value was measured against the lab, and is measured
again whenever the infrastructure in front of the service changes:

```bash
# Ordinary requests, until the limit answers 429.
for i in $(seq 1 400); do curl -s -o /dev/null -w '%{http_code}\n' https://<host>/ ; done | sort | uniq -c

# The same, each with a different forged address. If forging changes the
# answer, the hop count is wrong and the limit is not a limit.
for i in $(seq 1 400); do
  curl -s -o /dev/null -w '%{http_code}\n' -H "X-Forwarded-For: 203.0.113.$((RANDOM % 254 + 1))" https://<host>/
done | sort | uniq -c
```

Both runs must reach `429`. If the second one does not, set
`TRUSTED_PROXY_HOPS` to 1 in the environment's `.tfvars` and deploy.

**Measured on 19 September 2026**, against `3a24c19`, 300 requests per arm at
twenty at a time, the two arms run back to back so that the bucket refilled by
the same amount during each:

| Arm | Allowed | Refused | Duration |
|---|---|---|---|
| Ordinary requests | 133 | 167 | 4 s |
| A different forged `X-Forwarded-For` on every request | 134 | 166 | 4 s |

Identical. Forging the header buys a client nothing, which is the property the
limit depends on, and `TRUSTED_PROXY_HOPS = 2` is right for Cloud Run. Had the
value been wrong, the second arm would have been allowed throughout: every
request would have presented an address nobody had spent an allowance for.

## Asynchronous work

Cloud Run scales to zero, so there is no worker to hand work to. Cloud Tasks
holds the work and calls this same service back, and Cloud Scheduler does the
same for the periodic jobs (`docs/design.md`, section 2.5). Both stay inside
the free allowance at this volume: Cloud Tasks bills nothing for the first
million operations a month, and Cloud Scheduler allows three jobs per billing
account.

Three queues, one per class of work, so that a flood of notifications cannot
delay a payment webhook. Their retry policies differ for the same reason.

**The callbacks are signed.** Cloud Tasks and Cloud Scheduler mint an OIDC
token of the `marketplace-invoker` account; the endpoint checks that Google
signed it, that it was minted for this deployment's audience, and that it
belongs to that account and no other. A valid Google token proves who is
calling, not that they may.

**The audience is the platform's own host**, declared as a custom audience on
the Cloud Run service. The generated `run.app` address would be the obvious
choice and cannot be used: it only exists once the service does, so naming it
in that service's own environment is a resource referring to itself.

**To see that a scheduled job ran**, from Cloud Shell:

```bash
gcloud logging read \
  'resource.type=cloud_run_revision AND jsonPayload.message="job finished"' \
  --project=aleogr-marketplace-lab-a4j5 --limit=5 \
  --format='value(timestamp, jsonPayload.job, jsonPayload.took)'
```

The dispatcher runs every minute, so an empty answer means the schedule is not
firing or the callbacks are being refused — and a refusal says so in the log of
the service, with the reason.

**Read on 19 September 2026**, minutes after the first deployment that let the
callbacks through:

```
2026-09-19T05:22:03Z  dispatch-outbox  13.917183ms
2026-09-19T05:21:02Z  dispatch-outbox  13.670649ms
2026-09-19T05:20:02Z  dispatch-outbox  12.474012ms
2026-09-19T05:19:03Z  dispatch-outbox  12.300203ms
2026-09-19T05:18:03Z  dispatch-outbox  31.075831ms
```

One run a minute, each a few milliseconds on an empty outbox. The endpoint
refuses everything else: asked without a token, and with a made-up one, the
same address answers `401`.

**The first deployment did not let them through**, and every check was green
while the job silently never ran: the callback address was inside language
resolution and answered `302`, which Cloud Tasks and Cloud Scheduler do not
follow. An end-to-end test now asks for that address without following
redirects, which is the check that was missing.

## Sending e-mail

The provider is Brevo (`docs/requirements.md`, §25). Two credentials are
involved, and they are owned in opposite directions.

The **API key** belongs to the provider's account. Terraform creates the secret
that holds it and grants the service and the migration job read access to it,
but never the value: a key passed through a variable would be in the state, in
a plan output, and in whatever pasted it there. The owner adds it once:

```
printf %s "<the key from Brevo>" | gcloud secrets versions add mail-api-key \
  --project="$PROJECT" --data-file=-
```

The **webhook token** is the platform's own and is generated by Terraform into
`mail-webhook-token`, because nobody needs to choose it. It is read out once,
to be pasted into the provider's console beside the address the events go to:

```
terraform -chdir=infra/terraform output mail_webhook_url
gcloud secrets versions access latest --secret=mail-webhook-token --project="$PROJECT"
```

Brevo does not sign what it posts, so that token is the only thing in front of
the endpoint — which is why the platform **re-reads every event from the
provider's API** before acting on it. A forged bounce carrying the right token
suppresses nobody.

Until the key exists, the environment runs the fake adapter: `providers_mode`
is `fake`, and each message is written to a file in the container's `/tmp`
instead of being sent. Turning it to `real` in the environment's tfvars, with
`mail_from`, is what wires the key into the service.

**One e-mail, sent by hand**, is how a sending domain is proved to work once
its DKIM, SPF and DMARC records are published. It runs as the migration job
does — the same image, a different entry point:

```
gcloud run jobs execute marketplace-migrate \
  --project="$PROJECT" --region="$REGION" \
  --args=send-probe,<address>,pt-BR --wait
```

**Read on 19 September 2026**, minutes after the lab was switched to the real
provider. The probe ran as the migration job, with the arguments overridden for
that one execution, and finished:

```
Execution [marketplace-migrate-69rnh] has successfully completed.
```

The message arrived in the inbox — not in the spam folder — from
`marketplace@lab.aleogr.dev`, in Portuguese, with its subject and both parts as
the template renders them. That one e-mail is what proves the whole chain at
once: the key read from Secret Manager, a sender the provider accepts on the
strength of the authenticated domain alone, the signature, the template and the
language.

The starting of the execution took just under three minutes. That is
provisioning a task and pulling the image, not sending: the send itself is one
HTTPS call.

The two webhook addresses swapped when the mode changed, which is the probe
worth repeating after any change to that setting:

```
POST /webhooks/email/brevo     401   ← the configured provider, refused without a token
POST /webhooks/email/mailbox   403   ← no longer configured, refused by the CSRF guard
```

Neither redirects. A 302 there is an event that is silently never delivered,
with every check green — the failure this deployment has already had twice, on
the language prefix and on the callback endpoint.

### What the first bounce test taught

The webhook was configured in the provider's console — the address, the five
events this platform acts on, and the shared token as a custom header — and
then a message was sent to an address that cannot receive, to watch a bounce
travel the whole way. Three things came out of it, and two were defects.

**The confirmation never worked**, and it took two refusals to learn why. The
first was `400 invalid_parameter: End date should not be greater than current
date`, because the query asked for a window ending tomorrow; the second, after
that end date was dropped, was `400 missing_parameter: Start and end date both
required together`. So the provider takes both dates or neither, and any end
date computed here is tomorrow's for an account behind UTC during part of every
day — "current" is the account's time zone, which this process does not know.
The query therefore carries no window at all and relies on the provider's
default, which is the only form with no time zone in it.

The effect of the failure was not a missing feature but a queue storm: the
consumer returned an error, Cloud Tasks retried, and each retry failed
identically. No test saw any of it, because the stub that stands in for the
provider ignored the dates the real one validates — the fake was more
permissive than the service it represents. It now enforces both rules, with the
provider's own words, so either mistake fails the contract suite.

**A refusal is not a failure.** A provider that rejects a call rejects it
identically on the next attempt, so it is now told apart from a provider that
could not be reached: the first is said once, loudly, and not retried; the
second is retried. One wrong parameter used to become a retry for every event
that followed.

**Where a bounce comes from matters**, and none of this platform's own domains
produces the one that suppresses:

| address | what the provider records |
|---|---|
| anything `@lab.aleogr.dev` | `softBounces`, "Unable to find MX of domain" |
| anything `@aleogr.dev` | delivered — the domain is catch-all |
| anything `@id.aleogr.dev` | delivered — the alias domain is catch-all |

A soft bounce is transient: the provider retries it itself, and this platform
ignores it on purpose, because a full mailbox or a busy server is not a reader
to stop writing to. Sending to the no-MX domain three times did not make the
provider escalate it to `blocked` either. A hard bounce needs an address at a
domain that answers `550` for an unknown mailbox, which in the end meant an
unregistered address at a large provider — Google answers `550 5.1.1 The email
account that you tried to reach does not exist` in the handshake, so nothing is
delivered to anybody and the bounce is immediate.

**The provider spells one event four ways.** Its published specification names
the event `hardBounce` where a webhook subscribes to it and `hardBounces` where
its statistics are queried; the body it posts is documented as `hard_bounce`,
and that documentation is not in the specification. This platform had assumed
the third spelling, and a forged event written with that same assumption
confirmed nothing except the assumption. Event names are therefore matched by
their letters, singular — `hard_bounce`, `hardBounce` and `hardBounces` are one
event — because a bounce ignored over an underscore is a bounce nobody notices.

### What the lab proves, and what it does not

Proved against the deployed service, in this order:

- a real message delivered to a real inbox, in Portuguese, signed by the
  authenticated domain;
- the endpoint refuses a call with no token, with an empty token and with the
  wrong one, answers `202` to a call carrying it, and never redirects;
- it is mounted per provider: the address of the provider not in use is not
  served;
- a call that succeeds says so in the log, with the provider and how many
  events it carried, including none;
- the event reaches the outbox, the scheduled dispatcher hands it over within a
  minute, and the consumer runs;
- **the re-read decides**: an event the provider does not report is answered
  "not confirmed", ignored, and suppresses nobody — which is the whole reason
  it exists, since this provider does not sign what it posts
  (`docs/requirements.md`, §25).

And then the whole of it, on 19 September 2026, from one message sent to an
unregistered address at a large provider:

```
08:37:35  sending the delivery probe
08:37:36  the delivery probe was accepted
08:37:37  (the provider) hardBounces — 550-5.1.1 The email account … does not exist
08:37:38  mail events received                     ← the webhook, a second after the bounce
08:38:08  events handed to the queue               ← the scheduled job, the next minute
08:38:08  an address was suppressed    bounce  hard_bounce: 550-5.1.1 …
```

and then, six minutes later, the same message to the same address:

```
08:43:40  sending the delivery probe
08:43:40  a message was not sent to a suppressed address   probe
```

Three things only a real bounce could show. The confirmation works in the
affirmative — the query found the event at the provider and only then was the
address suppressed; until then every confirmation this platform had seen was a
negative one. The body the provider posts does spell the event `hard_bounce`,
which the code no longer depends on. And the reason kept beside the suppression
is the receiving server's own sentence, which is what answers "why did this
address stop receiving" months later.

The probe itself was the last thing this test corrected: it announced "the
delivery probe was accepted" for the message it had deliberately not sent.

## The key that protects the audit log

Every person whose data appears in the audit log has a key of their own, kept
in the database wrapped by a Cloud KMS key (`infra/terraform/audit.tf`). A
deletion request destroys the person's key: the records and the hash chain stay
exactly as they were, and what they hold about that person becomes unreadable
(`docs/requirements.md`, sections 21 and 18.3).

The service holds `roles/cloudkms.cryptoKeyEncrypterDecrypter` on that key and
nothing else. It can ask for a wrap and an unwrap; it cannot read the key,
create one or destroy one. The difference between that role and an
administrative one is whether a compromised revision can erase the platform's
ability to read its own audit log.

**A key ring cannot be deleted**, in any project, ever — Google keeps the name
taken. The lab is otherwise rebuilt from its configuration in minutes, and this
is the one resource that is not; it is declared with `prevent_destroy` so that
nothing tries.

The process proves the key at start-up, before it opens its port: it wraps a
value and unwraps it again. An identity that may encrypt and not decrypt looks
configured, serves happily, and loses everything it writes; one call at
start-up is what tells the difference. The start-up log says which keeper is in
use, and an environment with no key manager says so as a warning — what it
wraps with a key of its own is readable by anyone who can read its environment,
and what it wraps with a key it generated cannot be read after a restart at
all.

## The audit log

Three properties, each of which had to be built in rather than added later
(`docs/requirements.md`, section 21).

**Append-only, twice over.** The application role is granted everything on every
table by default, so the grant that runs after each migration takes `UPDATE` and
`DELETE` back on this one — and because that grant runs *after* the migrations,
a revoke written inside a migration would be undone by the next deployment. A
trigger refuses both again, for every role. The two are not redundant: with the
revoke removed the application role is stopped by the trigger, and with the
trigger removed the role that owns the table is stopped by nothing. Both were
checked that way.

**Tamper-evident.** Each record carries the hash of the one before it, over
every field including the ciphertexts — so altering what a record says about
somebody, even unreadably, breaks the chain. Fields are hashed with their length
in front of them, which is what stops content being moved across a boundary to
produce the same bytes. The chain is per marketplace, plus one for the platform:
a single chain would make every write in the system queue behind one row.

A Cloud Scheduler job walks every chain daily and **fails** when a record does
not verify, which is what makes a break something somebody hears about rather
than something a report would have shown if anybody had opened it.

**Erasable without a hole.** Records name people by internal identifier only.
The origin address is encrypted with the actor's key and the state before and
after with the **subject's**, which is the difference that makes a buyer's
deletion request reach what staff wrote about the buyer. Destroying a key leaves
the record, its position and its hash exactly as they were: the chain still
verifies and the content is gone.

## Schema changes that hide rows

A migration that puts a table under row-level security hides its rows from any
revision that reads it the old way — and the migration job runs **before** the
new revision serves, so for the couple of minutes between them the revision
still serving is the old one. In the lab, with no traffic and a service that
scales to zero, that window was accepted for the delivery that turned the
policies on.

For an environment with traffic it is two deployments, not one: first the code
that reads through the function, then the migration that turns the policies on.
The same rule covers dropping a column or renaming one — the schema and the
code that expects it are only ever in step if the change that hides something
comes second (`docs/roadmap.md`, F6).

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
