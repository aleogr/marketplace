# One database instance for both labs, asleep at night

Status: approved, not yet implemented. 2026-09-20.

This design covers two changes made together because they interact: moving the
lab databases of `marketplace` and `schooling` onto one shared Cloud SQL
instance, and stopping that instance outside working hours.

The decisions that survive implementation belong in `docs/infrastructure.md`;
this file is the record of how they were reached, and is written once.

`schooling` lives in another repository (`codeschool-ing/schooling`) and is not
this session's to write. Everything below that touches it is described so that
its owner can carry it out there.

## Why

The database is 96% of the gross bill. Read from the billing report for
21 August – 19 September 2026, filtered to Cloud SQL and grouped by SKU:

| SKU | usage | gross |
|---|---|---|
| Zonal — Micro instance | 861.7 hours | R$ 52.90 |
| Zonal — Standard storage | 11.56 GiB-month | R$ 11.69 |
| Storage PD Snapshot | 0.05 GiB-month | R$ 0.02 |
| nine network SKUs | 0 GiB | R$ 0.00 |
| | | **R$ 64.61** |

Two unit prices fall out of that table and everything below is derived from
them: **R$ 0.0614 per instance-hour** and **R$ 1.01 per GiB-month of disk**.

861.7 hours over 30 days is 1.2 instance-equivalents, so that total understates
the steady state. Both instances running continuously is 2 × 730 h × 0.0614 +
20 GiB × 1.01 = **R$ 110 per month**.

Nothing is being paid today: a Free Trial credit of R$ 1,659.21 covers it. That
credit **expires on 8 November 2026** rather than running out — at this burn it
would last over a year. So there is a date, not a shortage, and the work is
cheap to get wrong before it.

The target is **R$ 44 per month**: one instance awake 556 hours a month, one
10 GiB disk.

## Decisions

**D1. Consolidate by environment, never across environments.** Lab with lab,
production with production. Both instances here hold lab environments, so they
may share. The day either becomes production it leaves. This rule is why the
consolidation is allowed, and it is also the rule that would forbid it later.

**D2. The shared instance lives in a project of its own.** Not in either
tenant's project. In the alternatives, one project gains power of life and
death over the other's database and carries both costs on its bill. A neutral
home keeps the two tenants peers.

**D3. Share Cloud SQL and nothing else.** Consolidation pays only where a
resource has a **fixed per-instance cost that several tenants divide**. Cloud
SQL is that: the hour is charged per instance and one instance serves many
databases. Artifact Registry, Secret Manager, Cloud KMS, Cloud Storage and
Cloud Logging all charge per unit stored or per operation — the gigabytes move
with you, so consolidating them saves exactly nothing while costing isolation.
A shared registry would let one project's deployer delete the other's images; a
shared state bucket would let one `terraform apply` corrupt the other's state.
Memorystore, were it ever added, would be the next honest candidate.

**D4. The disk gets a ceiling.** `disk_autoresize_limit = 50`. Growth is
one-way — Cloud SQL never shrinks a disk — so an unbounded limit turns one
runaway write into a permanent bill. The `schooling` instance has
`disk_autoresize = true` with no limit today; that is true of it now, shared or
not.

**D5. Rehearse on the disposable database.** The `marketplace` lab rebuilds
itself from migrations and the seed in one deployment; `schooling` holds 42
tables and thousands of rows that do not. Everything new — the instance, the
cross-project IAM, the connector, the grants, the connection limits, the
activation policy — is proven with `marketplace` first. The rows that matter
travel a path that has already been walked.

**D6. Move the instance first; split `schooling`'s roles later.** Its owner
intends to adopt the migrator/service separation this repository uses. Doing
both at once means a failure could be either, with no way to tell which. The
move happens with its current permission model untouched.

**D7. The old instances are stopped, not deleted, for a week.** Stopped, each
costs only its disk — about R$ 10. Twenty reais buys a rollback that works, and
it is the cheapest insurance in the project.

**D8. Internal users are not a hard security boundary on a shared instance.**
A user created by `gcloud sql users create` belongs to `cloudsqlsuperuser`, and
a member of that role can grant itself back the `CONNECT` revoked below. The
grants and connection limits protect against **accident** — a runaway CI job, a
mistyped connection string — and not against deliberate action from inside.
Between two labs with one owner that is acceptable, and it is stated here
rather than implied away. It is a second reason for D1.

## Naming

Two principles, and the rest follows:

1. **The name states the environment, because the project id cannot be trusted
   to.** `aleogr-schooling` holds a lab and does not say so, and a project id is
   permanent. The instance name is where the environment is said out loud.
2. **Nothing in a shared resource's name may name a tenant.** A name mentioning
   `marketplace` or `schooling` would lie about what it serves.

| | name | why |
|---|---|---|
| projects | `aleogr-lab-<product>-<4>`, `aleogr-prd-<product>-<4>` | environment first, because everything sorts alphabetically in the console and in `gcloud projects list`. Grouping every lab together and every production together puts the thing you must not confuse at the front of the name. The random suffix is not decoration: project ids are globally unique and permanent. |
| shared project | `aleogr-lab-shared-<4>` | `shared` rather than `data`: it stays true as the contents grow beyond the database. |
| instance | `lab-postgres` | environment + engine, same rule. No major version in the name — a Cloud SQL instance cannot be renamed, and a deleted name is reserved for a time, so `postgres16-lab` would make a major upgrade into a rename that is not available. |
| databases | `marketplace`, `schooling` | the product, not the GCP project. No random suffix: a database name only has to be unique inside one instance you populated yourself, there is no collision to avoid, and the name appears in every connection string, every dump file and every error message. A second database for the same product takes a **purpose** suffix — `marketplace_analytics` — which says why it exists, where a random suffix says nothing. Underscore, not hyphen: a hyphen in a PostgreSQL identifier needs quoting forever. |
| roles | see below | a role belongs to the cluster, not to a database, so on a shared instance every tenant's roles share one namespace. `migrator` names a function rather than a tenant, which is a trap for the day a third tenant arrives. It becomes `marketplace_migrator`. |

The environment is not repeated in the database name: the instance already
carries it, and the instance is what is per-environment.

### The roles, in two steps

The cluster holds different roles before and after `schooling` adopts the
migrator/service separation (D6), and the table above would read as a single
end state without saying so.

| | at cutover | after the split |
|---|---|---|
| marketplace, migrations | `marketplace_migrator` | unchanged |
| marketplace, serving | the service account, as an IAM user | unchanged |
| schooling, migrations | `schooling` — one role does both | `schooling_migrator` |
| schooling, serving | `schooling` — one role does both | `schooling` |

So `schooling_migrator` does not exist on the day the data moves, and that is
deliberate: the move happens with the permission model `schooling` has today,
untouched.

When the split lands it is its own change, in its own repository, and it has
two parts beyond creating the role. The objects restored in Phase 2 are owned
by `schooling`, so ownership moves with `REASSIGN OWNED BY schooling TO
schooling_migrator`. And it is `schooling_migrator` that becomes the built-in
user created by `gcloud sql users create` — the one with the privilege to
create and drop tables — while `schooling` becomes a plain role holding
`SELECT, INSERT, UPDATE, DELETE` and nothing more. That is the same shape this
repository uses, and the reason is the same: a service that cannot drop a table
is a service whose compromise has a ceiling.

## The instance

`lab-postgres`, in `aleogr-lab-shared-<4>`, region `us-central1`:

- `POSTGRES_16`, edition `ENTERPRISE`, tier `db-f1-micro`, `ZONAL`
- 10 GB `PD_SSD`, `disk_autoresize = true`, `disk_autoresize_limit = 50`
- backups enabled, **window 12:00 UTC**, point-in-time recovery, 7 days of
  transaction log, 7 retained backups
- `ipv4_enabled = true`, `ssl_mode = ENCRYPTED_ONLY`, no authorized networks
- database flag `cloudsql.iam_authentication = on`, which the marketplace
  service needs and which password users are unaffected by
- `deletion_protection` and `settings.deletion_protection_enabled` both on

The backup window is 12:00 UTC (09:00 local) and not the small hours **because
a stopped instance runs no automated backup**. With the sleep schedule below
and the windows where they are today, both projects would simply stop having
daily backups, silently, with no error to notice.

## Isolation

By default in PostgreSQL every role may connect to every database in the
cluster: `PUBLIC` holds `CONNECT` on all of them. Without the revoke, the
`schooling` role reaches the `marketplace` database.

```sql
REVOKE CONNECT ON DATABASE marketplace FROM PUBLIC;
REVOKE CONNECT ON DATABASE schooling   FROM PUBLIC;
GRANT  CONNECT ON DATABASE marketplace TO marketplace_migrator, "<service-account>";
GRANT  CONNECT ON DATABASE schooling   TO schooling;

ALTER DATABASE marketplace CONNECTION LIMIT 10;
ALTER DATABASE schooling   CONNECTION LIMIT 10;
```

When the split lands, `schooling_migrator` joins the `GRANT CONNECT` for the
`schooling` database.

The connection limits are what keep a burst from this repository's CI from
leaving `schooling` unable to connect — the 0.6 GB of shared RAM answered by
configuration rather than by a larger tier. Subject to D8.

## The sleep schedule

The owner works on both projects together, never one alone, between 08:00 and
22:00 on weekdays and at any hour at weekends. So the instance sleeps only on
weeknights, and the weekend runs unbroken from Friday evening to Monday night.

```
start  07:45  Tue–Fri   45 7 * * 2-5
stop   22:00  Mon–Thu    0 22 * * 1-4
time_zone = "America/Sao_Paulo"
```

07:45 rather than 08:00 because a stopped instance takes a minute or two to
accept connections. The zone by name rather than a UTC conversion, so the cron
stays correct if Brazil ever restores summer time.

Friday night is left awake: two hours until Saturday is not worth a stop and a
start. The same for Sunday night, which would also risk the database dying at
midnight while somebody is working.

About 39 hours asleep a week. 557 awake hours a month × R$ 0.0614 + R$ 10.11 of disk
= **R$ 44 per month**, against R$ 110 today.

### What runs inside the window and must move

| what | today, local | whose | becomes |
|---|---|---|---|
| `schooling` automated backup | 04:00 | theirs | 12:00 UTC |
| `marketplace` automated backup | 03:00 | ours | 12:00 UTC |
| `schooling-analyse-nightly` | 03:10 | theirs | 08:10 |
| `schooling-settle-nightly` | 03:40 | theirs | 08:40 |
| `verify-audit-chain` | 01:17 | ours | 12:17 UTC |

There is no transaction log for the hours an instance was stopped, so the
`schooling` restore drill must target a moment the instance was awake. That
belongs in a note in its own repository.

### Deploying into the window

A merge to `main` at 23:00 runs the migration job and `make e2e-lab` against a
stopped database, and fails. The deployment workflow starts the instance and
waits for it before migrating. The instance then stays up until the next stop,
which costs a few hours at worst and removes the trap.

Whether `schooling`'s pipeline touches the live instance on deploy was not
established from its repository; its CI uses a `postgres:16-alpine` service
container and does not.

## The migration

### Phase 0 — build the new home

Create the project, its Terraform state, and the instance. Nothing moves; both
existing instances keep serving. Stopping here costs one extra instance for a
few days.

### Phase 1 — rehearse with `marketplace`

Create the `marketplace` database, `marketplace_migrator`, the IAM user for the
service account, the grants and the connection limits. Point this repository's
`DATABASE_INSTANCE` and `DATABASE_NAME` at the new instance. The deployment
does the rest: the migration job builds the schema, seeds the marketplaces and
writes their audit records, and `make e2e-lab` says whether it worked.

If anything is wrong, one variable goes back and the old lab serves again. No
data is at risk and the whole path is proven.

### Phase 2 — move `schooling`

Only after Phase 1 is green.

1. **Freeze.** Pause both nightly schedulers, deploy nothing, and set the
   source database read-only. Without a freeze, a write between the dump and
   the cutover disappears leaving no trace.
2. **Photograph the source** with `tools/restore-drill/verify.sql`.
3. **Copy.** `pg_dump --no-owner` from the source, `psql` into the target,
   through the Cloud SQL Auth Proxy. Chosen over `gcloud sql export/import`
   because what is being restored can be read, and `--no-owner` decouples the
   dump from the role names, which are what is changing. `pg_trgm` — from
   migration 0048 — arrives as a `CREATE EXTENSION` in the dump, which a user
   created by `gcloud sql users create` has the privilege to run.
4. **Photograph the target** with the same file.
5. **Diff the two reports, less the `snapshot|` line** — the one line allowed
   to differ. Identical, or stop. There is no third outcome and no "close".
6. **Cut over.** A new version of the `schooling-database-url` secret pointing
   at the new instance's socket, then deploy and smoke-test.
7. **Unfreeze** the schedulers.

`verify.sql` is the right instrument and is already written: it produces a
report rather than a verdict, discovers its tables from `information_schema`
so a migration merged tomorrow is covered without anybody editing it, and
reads everything in one `REPEATABLE READ` snapshot.

### Phase 3 — cool down

Both old instances stay **stopped for a week**, then are deleted (D7).

### Phase 4 — sleep

The schedule above is applied to the new instance, with the five timing changes
already made.

## Out of scope

- Artifact Registry cleanup policies. Image storage is the only other cost with
  a permanent footprint and it grows with every deploy, but the answer there is
  deletion, not sharing (D3). A separate piece of work.
- Splitting `schooling`'s single role into a migrator and a service (D6).
- `codeschool-ing`, a project on the same billing account, outside the
  organization, with no Cloud SQL instance. Unlinking or deleting it is
  independent of all of this.

## What the owner executes

Manual steps are given one at a time, and steps inside `codeschool-ing/schooling`
are given as prompts to paste into that project's own session, at the moment
each is needed rather than all at once. In order: the five timing changes and
the disk ceiling (Phase 0, unhurried); the freeze and the secret cutover
(Phase 2); the restore drill note.
