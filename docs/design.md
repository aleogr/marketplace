# Marketplace Platform: Design

> **Status:** design document produced from the design session of 2026-09-17. It closes the open topics of `docs/requirements.md` section 29 that could be closed, records the reasons, and lists what remains open and what is needed to close it. It is the input for the roadmap, which is written in a separate session.
>
> **Sources of truth:** `docs/requirements.md` decides *what*; this document decides *how*. Where a decision here changed a requirement, the requirement was updated in the same pull request. The research documents in `docs/research/` explain *why* a decision was taken and are cited where useful.
>
> **Conventions:** section numbers in the form §N refer to `docs/requirements.md`. Provider names appear only where the design depends on a documented capability; the design works with any provider on the shortlists.

---

## 1. Scope of the MVP

### 1.1 What is in

Two marketplaces with different transaction profiles, one platform.

**Electronics marketplace**
- Catalog with a category tree, attributes per category, product/offer separation with GTIN matching and a Buy Box (§8.2).
- Search and filters over attributes, standard sort options, product comparison of three to four products (§8.3, §9).
- Multi-store cart with a single payment, PIX and credit card with installments, split to each store and to the platform, payout held until delivery (§10, §11).
- Label purchase and tracking through the store's shipping account, returns by Correios reverse code (§12, §14).
- Product and store reviews, quantitative store reputation with graded sanctions (§16, §20).
- Public questions and answers on listings and private messages, both with contact-data detection (§15).

**Vehicles marketplace**
- Paid listings with exposure tiers and duration as the revenue model, plus plans for dealers (§4).
- Public contact on the listing at the store's choice; no contact-data detection in this marketplace (§15).
- Optional online reservation per listing: the buyer pays a reservation through the platform, held until both parties confirm the handover; no cart, no shipping (§10).
- Public questions and answers and private messages.

**Common**
- Accounts separate per marketplace, staff accounts on the platform with roles scoped by marketplace (§4, §19).
- Store onboarding through the payment gateway for every store in every marketplace, including stores that only buy listings: the gateway's KYC state is the "verified identity" badge (§11, §20).
- Pre-publication moderation with automatic pre-checks and a manual queue (§8.5).
- Auditing, LGPD self-service, two-factor authentication, in-app and e-mail notifications, en-US and pt-BR (§6, §17, §18, §21).

### 1.2 What is out of the first release, with the model ready

Discount coupons; paid listings in the electronics marketplace; pickup at agencies and lockers as a delivery type chosen by the buyer; automatic translation of listings; recommendations beyond "customers who bought this also bought"; web push, SMS and WhatsApp; subscription plans for electronics stores; full in-platform payment of a vehicle; pickup at the store's address; markets outside Brazil and cross-border sales; self-hosted video; a platform-run identity verification provider; export of behaviour events to an analytics warehouse.

Each of these has its place in the data model (section 5) so that adding it later is an addition, not a redesign.

## 2. Application architecture

### 2.1 Shape

A **modular monolith in Go, one binary, one repository** (§24). Modules are organised by domain, not by technical layer. Each module owns its tables, exposes a service behind an interface, and is tested on its own. Modules that belong to the same aggregate call each other directly inside one transaction; modules that do not communicate through domain events written to an outbox.

Modules:

| Module | Owns |
|---|---|
| `identity` | users, credentials, sessions, second factors, recovery, roles, permissions |
| `tenancy` | markets, marketplaces, parameters, fee schedules, listing products and plans |
| `catalog` | categories, attributes, products, variants, offers, media, moderation, questions and answers |
| `search` | indexing and querying of offers, aggregates for sorting |
| `listings` | paid listing purchases and their lifecycle (vehicles first) |
| `checkout` | carts, stock reservation, checkout, installment plans |
| `orders` | orders, shipments, fiscal documents, returns, disputes, evidence |
| `payments` | gateway port, payments, split lines, payout holds, refunds, chargebacks, vehicle reservations |
| `shipping` | shipping port, quotes, labels, tracking, reverse requests |
| `messaging` | message threads, reports |
| `reputation` | reviews, store replies, reputation snapshots, sanctions, appeals |
| `notifications` | in-app and e-mail notifications, preferences, suppression list |
| `audit` | append-only, hash-chained audit log with per-user encryption keys |
| `analytics` | behaviour events and daily aggregates |
| `console` | administration screens over the other modules |

### 2.2 Ports and adapters

External integrations sit behind interfaces defined by the domain (§25): `payments`, `shipping`, `email`, `geoip`, `translation`, `search`, `identityverification` and `breached` (whether a password appears in a public breach; Pwned Passwords, failing open). Each port has one real adapter and one fake used by tests. The `identityverification` port is satisfied in the MVP by the payment gateway's KYC; a dedicated provider can be added later without touching the store onboarding flow. The `search` port is satisfied by PostgreSQL; a dedicated engine can replace it later.

Webhooks arrive at one handler per provider. The handler verifies the signature where the provider signs, or the shared token otherwise, stores the raw event with the provider's event id as a unique key, answers immediately, and enqueues processing. The processor re-reads the object from the provider's API before acting, applies the state transition, and writes the resulting domain events to the outbox in the same transaction.

### 2.3 Request pipeline

Every request passes, in order: marketplace resolution by host (§7); language resolution by URL prefix, cookie or geolocation (§6); session; tenant context, which sets the database session variable used by row-level security; the handler. Middlewares mounted once apply the `noindex` header when indexing is off (§7.1), security headers, rate limiting on sensitive endpoints, CSRF protection and the origin data used by auditing (§24).

### 2.4 User interface

templ renders every page and every fragment; HTMX drives interaction; Alpine.js is used only in declared components: multi-photo upload, forms whose attribute fields change with the category, tables with filters (§24). The rule is one HTMX interaction, one route, one templ fragment; the UI never consumes JSON. Static assets are embedded with `go:embed` and served with a content hash in the file name for long caching.

Dashboard patterns (closes the §29 item on HTMX and Alpine.js):
- Tables with filters and pagination are fragments loaded with `hx-get` and the query string, with `hx-push-url` so the state is in the URL.
- Category-dependent forms load the attribute fields as a fragment when the category changes.
- Multi-photo upload uses Alpine to hold client state and progress and uploads directly to the bucket through signed URLs; the server records the object afterwards.
- Validation always runs on the server; client-side validation is a convenience only.

### 2.5 Asynchronous and scheduled work

Cloud Run scales to zero, so there is no always-on worker. The pattern is **outbox + Cloud Tasks + Cloud Scheduler**:
- Every operation writes its domain events to the `outbox_event` table in the same transaction as its data.
- A dispatcher delivers each event to a Cloud Tasks queue (one queue per class of work: webhooks, notifications, jobs), with an idempotency key. Cloud Tasks calls back an internal endpoint of the same service, authenticated with an OIDC token of the invoker service account.
- Cloud Scheduler triggers the periodic jobs through the same endpoint: tracking reconciliation, expiry of stock reservations, vehicle reservations and labels, payout releases, audit chain verification, media processing retries. Jobs take a lock keyed by job name so two runs never overlap.

Alternatives rejected: in-process goroutines with a minimum instance (pays for an idle instance and loses in-flight work on restart); Pub/Sub (more parts for the same outcome at this scale).

### 2.6 Tenancy isolation

Two layers, closing the §29 item on isolation:
- **Application:** the marketplace is part of every request's context; every repository query is scoped by it. Authorization between stores uses the store id of the session and is covered by dedicated tests.
- **Database:** every tenant table carries `marketplace_id`; the application sets a session variable when it opens a transaction, and row-level security policies refuse rows of other marketplaces even when a query forgets the filter. A transaction that named no marketplace reads nothing, rather than everything. Host resolution is the one thing that runs before a marketplace is known — it is what decides which one — and reads the routing map through a single function that runs as the owning role, so the routing tables are covered by the policies like any other. Staff read tenant data under the same policies: a console route names the marketplace, authorization checks that the staff member's roles grant the permission in that marketplace or across the platform, and only then is the transaction opened for it; a view across the platform walks the permitted marketplaces one transaction at a time, so a missing filter can never show another marketplace's rows (§19; decided with the owner on 2026-09-26, replacing an earlier plan of a database role without RLS for staff). Staff accounts, which belong to no marketplace, are the rows a transaction that names no marketplace sees in the identity tables.

### 2.7 Search

PostgreSQL only in the MVP (closes the §29 item on search): filterable attribute values live in a JSONB column with a GIN index; full-text search uses `tsvector` configured per language; "contains" matching uses `pg_trgm`; the aggregates that sorting needs (sales count, rating average) are materialised by a job. Zero fixed cost and one source of truth, enough for tens of thousands of listings. The `search` port allows a dedicated engine when relevance or volume demand it.

### 2.8 Testing

Unit tests per module; integration tests against a real PostgreSQL with migrations applied and RLS active; end-to-end tests with **Playwright in Python** for the critical flows (sign-up, listing creation, search, checkout with the fake gateway, store panel), because the Claude Code environment already has Playwright and Chromium and the same tests produce the verification evidence required by the definition of done (§3). The E2E suite lives in `e2e/` with pinned dependencies; CI starts PostgreSQL as a service container, builds the binary, starts it with fake providers and runs the scenarios; screenshots are kept as CI artefacts. This closes the §29 item on the E2E language and runner.

## 3. Infrastructure

Everything is declared in Terraform in one GCP project in `us-central1` (§26).

- **Cloud Run:** one service, scale to zero, high concurrency per instance. Environment variables declared in Terraform with explicit values (`INDEXABLE`, active providers, fake or real mode). Secrets live in Secret Manager and are mounted as variables.
- **Cloud SQL (PostgreSQL):** smallest instance, automatic backups, point-in-time recovery on, IAM authentication through the Cloud Run connector. Targets (closes the §29 item on backups): recovery point objective 5 minutes through PITR, recovery time objective 4 hours, backup retention 30 days, a restore test every quarter into a temporary instance, with the date of the last test recorded as a console parameter. Those targets are **production's**; an environment that is rebuilt from the repository keeps a shorter window, and records why (`docs/infrastructure.md`).
- **Cloud Storage:** a private bucket for original uploads and a public bucket for image derivatives only. Object versioning with a 30-day lifecycle rule for old versions is the retention and recovery approach for media. Uploads go directly to the bucket through signed URLs.
- **Cloud Tasks and Cloud Scheduler:** as in section 2.5; both within the free quota at MVP volume.
- **Service accounts:** separate for Cloud Run, for the Tasks and Scheduler invoker, and for CI/CD through Workload Identity Federation restricted to this repository; least privilege everywhere (§26, §27).
- **Domains:** the lab uses Cloud Run domain mapping with Cloudflare in "DNS only" mode (§7). Production domains remain open (section 7) with a scheduled spike; nothing in the design depends on the outcome, because the service resolves the marketplace by the host it receives.
- **Observability:** structured JSON logs to Cloud Logging, Cloud Run's built-in metrics, e-mail alerts on 5xx rates and on failed jobs. No paid APM.
- **CI/CD** (closes the §29 item): GitHub Actions. On every pull request: `go vet`, `staticcheck`, `golangci-lint`, `gosec`, `govulncheck`, `gitleaks`, unit and integration tests with PostgreSQL as a service container, E2E tests, image build scanned by Trivy. On merge into `main`: build, push to Artifact Registry tagged with the commit, deploy to the lab. On a release tag: **promote the same image by digest**, without rebuilding, to production, and create the GitHub Release (§27). Database migrations run as a Cloud Run Job before the deployment in the same pipeline.
- **Fixed costs** (closes the §29 item): Cloud SQL is the only material permanent cost (the smallest instance, tens of dollars per month). Cloud Run, Tasks, Scheduler, Storage and Logging stay within free quotas or cents at initial volume. Payment, shipping and e-mail providers on the shortlists have no monthly fee. A load balancer, if adopted for production domains, would be the second fixed cost.

## 4. Data model (high level)

General rules: every tenant table has `marketplace_id` under row-level security; money is stored in minor units with a currency code (§6); every external identifier is stored as the pair `(provider, external_id)` (§25); timestamps are UTC; audit tables are append-only.

**Platform and tenancy**
- `market`: currency, enabled payment methods, tax and consumer rules as data with effective dates, address format. Brazil is the first row, not a special case (closes the §29 item on markets). Currencies: BRL only in the MVP, no conversion; every amount already carries its currency (closes the §29 item on currencies).
- `marketplace`: hosts, market, languages, revenue model (commission or paid listings), flags such as "contact-data detection" and "reveal contact", plus versioned entries in `parameter` (key, value, effective date, author).
- `fee_schedule` and `fee_rule` (effective date, category, percentage, fixed fee per item or per order, minimum, price bands); `listing_product` (paid listing types: price, duration, exposure) and `plan` (dealers).

**Identity**
- `account` (per marketplace; staff belong to the platform with a null marketplace; named `account` rather than `user`, a reserved word in PostgreSQL), `session`, `credential`, `second_factor` (type, encrypted secret), `recovery_code`, `email_code`, `sign_in_challenge` (the two-step sign-in, separate from `session`), `role`, `permission`, `role_scope`.
- `store` (type: private, company, dealer; ICMS-taxpayer flag; "new store" state; current reputation), `store_member`, `store_provider_account` (gateway or shipping: provider, external id, onboarding state, encrypted token).

**Catalog**
- `category` (tree, translations), `attribute` (type, unit, translations), `category_attribute` (required, filterable, comparable, variant-defining).
- `product` (catalog item, GTIN, original language, translations), `product_variant` (combination of variant-defining attributes), `product_media`.
- `offer` (store × variant: price, condition, stock or availability for unique items, weight and dimensions, moderation state, badges with expiry). A vehicle listing is an offer of a single-variant product. `offer_attribute_value` in indexed JSONB for filters.
- `moderation_case` (queue, decisions, reasons), `question`, `answer`.

**Commerce**
- `cart`, `cart_item` (per marketplace, for a visitor or a user); `stock_reservation` with expiry.
- `order` (snapshot of address and amounts), `order_item`, `shipment` (per store: quoted service, price, deadline range, label, label expiry, state, post-delivery adjustment), `shipment_volume`, `shipment_event` (raw tracking from webhooks), `fiscal_document` (NF-e key or product data for the content declaration), `return_request` (reverse code, state), `dispute`, `evidence_item`.
- `payment` (provider, external id, method, installments, state), `payment_split_line` (recipient: store or platform; kind: item, shipping, commission, adjustment), `payout_hold` (custodian, release event, deadline), `refund`, `chargeback`.
- `vehicle_reservation` (listing, buyer, amount, deadline, confirmations by each party, state), `listing_purchase` (a store's purchase of a paid listing), `coupon` and `discount_line` (with the funder: platform or store), present in the schema without user interface.

**Social and trust**
- `review` (product or store, source order, media, moderation state), `store_reply`, `reputation_snapshot` (metrics per window), `sanction` (kind, reason, deadline, appeal), `message_thread`, `message`, `report`.

**Cross-cutting**
- `notification`, `notification_preference`, `email_suppression`; `outbox_event`, `inbound_webhook` (provider, unique event id, payload, state); `audit_log` (append-only, hash-chained, personal fields encrypted with a `user_key` kept outside the log); `behavior_event` (partitioned by month); `translation` (key, language, text) for categories and institutional texts.

## 5. Critical flows

**Electronics checkout.** Cart per marketplace → early shipping estimate (a quote per store with the store's shipping token, cached briefly per postal code and volume) → checkout: revalidate price, stock and availability; reserve stock for N minutes (`stock_reservation`, released by a job on expiry); compute the installment plan from the platform's tables; create the order and the payment with its split lines (items and shipping to each store, commission to the platform); create the charge at the gateway (PIX with QR code and expiry, or a card tokenized in the browser). Payment confirmed by webhook → order paid, stock decremented, a `payout_hold` per store (custody in the store's sub-account or in the platform balance, as the provider allows), notifications. Failure or expiry → reservation released, order cancelled.

**Shipping and release.** The store sees the order in its panel, attaches the NF-e or confirms the data for the content declaration, buys the label from its own wallet through the platform's panel, prints it and drops the parcel off. Tracking events arrive by webhook as `shipment_event`; "delivered" starts the payout hold countdown; a job releases the `payout_hold` by calling the gateway (release the escrow or transfer); "undelivered", "returned" and silence for X days raise alerts. A daily reconciliation queries the provider for shipments without new events.

**Return.** The buyer opens the request within the window; the store accepts or the console decides; the platform requests the reverse code from the shipping provider with the store's token; the buyer receives the code and the declaration; delivery of the reverse confirmed → refund through the gateway (partial or total), split reversed, commission returned according to the rule in section 6.

**Vehicle reservation.** The buyer clicks "reserve" on a listing with reservation enabled → single-item checkout without shipping → payment captured and held → listing "reserved", contact details visible on the order, the N-day deadline runs. Each party confirms the handover in its panel; two confirmations → release to the store minus the commission (default zero); withdrawal by either party or deadline expiry → refund and the listing returns to "available"; one confirmation only → a dispute.

**Paid vehicle listing.** The store chooses the listing type and duration → `listing_purchase` → direct charge to the platform's own sub-account (PIX or card, no split) → the listing enters pre-publication moderation → published with an expiry; a job expires it and notifies the store.

**Store onboarding.** Sign-up → terms acceptance → gateway: sub-account created through the API and documents collected through the provider's hosted link; state synchronised by webhook; "active" grants the verified-identity badge → shipping: OAuth authorization at the shipping provider, token stored encrypted; wallet verification → first listing released to moderation. A vehicles store that only buys listings skips the shipping step.

**Webhooks and outbox.** As in section 2.2. The outbox dispatcher sends each event to Cloud Tasks with an idempotency key; consumers are idempotent by event id.

**Pre-publication moderation.** Listing saved → automatic pre-checks (category plausibility from required attributes, contact data in the text where detection is on, price outside the category's band or far from the FIPE reference for vehicles) → prioritised queue → approved, returned with a reason, or refused; every decision audited; stores with a clean history enter "fast approval" (a parameter). This closes the §29 item on how moderation works.

## 6. Decisions taken in this session

Each decision names the §29 topic it closes and the reason.

1. **MVP boundary** (§29 "MVP boundary"): section 1. Public questions and answers are in the first release because they reuse the messaging mechanism. Coupons, paid electronics listings, pickup points, automatic translation, advanced recommendations, web push, an own identity provider and store plans are out, with the model ready.
2. **Vehicles: revenue from listings, contact visible, reservation optional** (§29 "messaging rules", §4, §10, §15). Contact masking is porous (a phone number in a photo, a creative spelling, "call me on WhatsApp" in chat); a revenue model that depends on hiding contact is fragile, and no vehicle competitor charges a commission. The vehicles marketplace therefore launches with paid listings and dealer plans, shows contact at the store's choice, keeps online reservation as a trust feature the store enables per listing, and sets the commission on the reservation to zero by default. Contact-data detection is a per-marketplace flag, off for vehicles. Requirements updated: §4, §10, §15.
3. **Vehicle reservation economics** (§29 "vehicles checkout details"): amount as a percentage of the listed price with a floor and a cap (defaults 2%, R$ 500, R$ 5.000), deadline N days (default 7) for both parties to confirm the handover, full refund on buyer withdrawal while the right of withdrawal for vehicles is not validated by the lawyer (forfeiture is a parameter, off), full refund and a cancellation mark on the store's reputation on store withdrawal, automatic refund on deadline expiry without confirmations, a dispute when only one party confirms. All values are marketplace parameters. Requirements updated: §10.
4. **Commission base** (§29 "coupon funding and the commission base"): the base is the item amount actually paid by the buyer to the store, after store-funded discounts (store coupons, a PIX discount the store chose to give); platform-funded discounts do not reduce the base; shipping charged to the buyer is outside the base because it is a pass-through cost and the store is the shipper. The commission is computed at payment time as a split line. On a partial cancellation the commission of the cancelled item is returned proportionally; non-refunded processing fees stay with the store by default. Coupons can be funded by the platform or by the store, recorded on the discount line. Requirements updated: §4, §13, §14.
5. **Two-factor authentication per user type** (§29 "mandatory 2FA methods"): owner and staff mandatory with TOTP or a security key (e-mail codes are not a second factor for console users, because e-mail is the recovery channel); stores step-up on sensitive actions (bank or payout account changes, e-mail and password changes, bulk label generation, terms acceptance) with any method, e-mail code as the default for a new store; buyers optional, with step-up by e-mail code when adding a card or changing e-mail. Recovery: one-time recovery codes generated when TOTP or a key is enrolled; staff recovered only by the owner or another administrator resetting the factor, audited; stores and buyers recovered by e-mail with a security delay (default 24 hours), notification on every channel and revocation of sessions. Requirements updated: §18.2.
6. **End-to-end tests** (§29 "language and execution of end-to-end tests"): Playwright in Python, section 2.8.
7. **Verified identity through the gateway for every store** (§29 "platform-level identity verification"): every store in every marketplace completes the gateway onboarding, including stores that only buy listings; the sub-account is free at the shortlisted providers and their verification is biometric; the KYC state is the badge. No new provider, no cost per check; a dedicated provider remains pluggable behind the `identityverification` port. Requirements updated: §20.
8. **Asynchronous work, search, isolation**: sections 2.5, 2.7 and 2.6.
9. **Markets and currencies** (§29 "modeling markets", "currencies and price conversion"): section 4, `market`.
10. **Language URL format** (§29): BCP 47 prefix with canonical case, `/en-US/...` and `/pt-BR/...`; lowercase variants redirect with 301; the root without a language performs detection; `hreflang` and `x-default` on every page; one sitemap per language; the remembered choice lives in a one-year cookie.
11. **IP geolocation** (§29): GeoLite2 downloaded at process start from a private bucket and cached locally; a weekly GitHub Action with the MaxMind licence in the repository's secrets refreshes the bucket; nothing in the binary, no runtime cost. Visitor IP: on Cloud Run the last address in `X-Forwarded-For` is the one appended by Google's front end; the number of trusted proxies is a parameter for the day Cloudflare proxies traffic.
12. **Media** (§29): signed-URL upload, type and size validation, processing in a job (three sizes, WebP, EXIF removed), per-category limits as parameters (defaults: 12 photos for electronics, 20 for vehicles, 10 MB per file, 1 video link), video by YouTube or Vimeo link with a cookie-free embed loaded on click (§8.4).
13. **Buy Box** (§29): among offers of the same variant with stock, from active stores above the minimum reputation: lowest total to the buyer (price plus estimated shipping to the buyer's postal code when known, price alone otherwise); ties by reputation, then by delivery deadline. New or sanctioned stores never win but appear under "other offers". Weights are parameters.
14. **Metrics and behaviour events** (§29): `behavior_event` in PostgreSQL, partitioned by month, written by a job from the outbox rather than in the request; raw retention 90 days with daily aggregates kept; the console reads aggregates only. Export to an analytics warehouse (pay per query, scales to zero) is a later phase.
15. **Console parameters** (§29): a behaviour becomes a parameter when it changes with operation, not with code. Initial list: fee schedule, paid listing types, media limits, payout hold lengths by reputation band, reputation metrics and thresholds, sanctions and appeal deadlines, the "new store" state, vehicle reservation rules, contact flags per marketplace, rate limits, stock reservation expiry, review windows.
16. **Marketplace creation flow** (§29): a console wizard: data and languages → root categories → revenue model → parameters copied from a template → infrastructure checklist executed one step at a time with an instruction for the owner (DNS in Cloudflare, domain mapping, e-mail domain records and a verified sender). The marketplace leaves "in preparation" only when the host answers and the e-mail domain verifies.
17. **Messaging retention and moderation** (§29): messages and questions kept for two years after the last order between the parties (the dispute and chargeback horizon), then anonymised; reports go to the moderation queue; personal content encrypted with the user key as in the audit log, so LGPD deletion works by key destruction.
18. **Dashboards with HTMX and Alpine.js** (§29): section 2.4.
19. **Backups** (§29): section 3.
20. **CI/CD and image promotion** (§29): section 3.
21. **Fixed costs** (§29): section 3.
22. **Provisional retention defaults** (§29 "auditing versus privacy"): user keys destroyed five years after the last order or 30 days after a deletion request with no pending obligation, whichever is later, until the lawyer sets the periods.

## 7. What remains open and what closes it

| Topic (§29) | What is needed |
|---|---|
| Right of withdrawal for vehicles and its effect on the reservation | Lawyer's opinion; until then the reservation is always refundable |
| Retention periods for audit and messages | Lawyer's opinion; provisional defaults in section 6 |
| Security incident response duties and deadlines | Lawyer's opinion; the runbook structure (detect, contain, revoke sessions and keys, assess, notify) and the incident record in the console are designed |
| Installment cost tables and how the seller-funded cost is expressed | The chosen gateway's commercial proposal; the platform's plan model accepts a monthly rate or tiers |
| Maximum amount per transaction and per method for vehicle reservations | The gateway's commercial proposal; the reservation cap parameter must stay below it |
| Payment gateway selection | Commercial proposals from Pagar.me and Asaas (PagBank if multi-store carts are possible); criteria and questions in §29 |
| Shipping provider confirmation | Written confirmation from Melhor Envio of the multi-store model and partner terms; SuperFrete is the second adapter |
| Production domains | A spike before launch comparing a global HTTPS load balancer with a serverless NEG (about US$ 18 per month plus managed certificates, wildcard possible) and Cloudflare as a proxy in front of the `run.app` URL (no cost, host rewrite by a Worker) |
| Phases and risks for future markets | Mapped at high level in section 10; detailed when the first foreign market is chosen |

## 8. Actions that depend on the owner, with the recommended moment

| Action | Recommended moment |
|---|---|
| Engage the lawyer (terms of use, right of withdrawal for vehicles, retention periods, incident duties) | Now; answers needed before the checkout phase reaches the lab with real stores |
| Request commercial proposals from Pagar.me and Asaas (and PagBank about multi-store carts) with the questions in §29 | During the foundation phase, so the answers exist when the payments phase starts |
| Open sandbox accounts at the chosen gateway | Start of the payments phase |
| Obtain Melhor Envio's written confirmation of the multi-store model and partner terms; create the OAuth app and the sandbox account | Start of the shipping phase |
| Create the e-mail provider account (Brevo, §25), verify the platform sending domain, publish its DKIM, SPF and DMARC records | Foundation phase, because sign-up and password recovery depend on e-mail |
| Create the MaxMind account and GeoLite2 licence | Foundation phase |
| Run the production-domain spike | Before the launch phase |
| Provider homologations and the manual test plan with real amounts and parcels | Launch phase |

## 9. Risks

- **A gateway refuses the model** (hidden fixed cost, a per-transaction limit too low for vehicle reservations): mitigated by the provider-neutral design and the second option on the shortlist.
- **Melhor Envio does not confirm the multi-store model in writing:** SuperFrete uses the same OAuth model; its adapter is the second to write.
- **Cloud Run domain mapping in production:** known risk, spike scheduled, load balancer as the fallback with a fixed cost.
- **Vehicles revenue depends on paid listing volume without a commission:** a business risk, not a technical one; parameters allow a commission on reservations later.
- **Manual moderation volume:** automatic pre-checks and fast approval for clean stores reduce it; without them the queue becomes the bottleneck.
- **One binary and one person:** operational complexity is concentrated; runbooks, alerts and the manual test plan mitigate it.
- **Fiscal rules change** (the electronic content declaration became mandatory in April 2026 and import taxes changed in May 2026): rules are data with effective dates, never code.

## 10. Phases at high level

Input for the roadmap; each phase depends on the previous ones unless stated. The detailed
roadmap of the phase under way lives in `docs/roadmap.md`.

1. **Foundation:** repository, CI/CD, Terraform, Cloud Run and Cloud SQL in the lab, tenancy with row-level security, identity with two-factor authentication, roles and permissions, auditing, internationalization with both languages, e-mail, outbox and jobs.
2. **Catalog and search:** categories and attributes, products and offers, media, pre-publication moderation, search and filters, comparison, public questions and answers.
3. **Stores and trust:** store onboarding with the gateway (KYC and badge), reputation, sanctions, messaging. Depends on the gateway choice for the onboarding adapter.
4. **Vehicles:** paid listings, dealer plans, visible contact, online reservation. Depends on phase 3 and on the gateway.
5. **Checkout and payments:** cart, checkout, split, payout hold, installments, refunds. Depends on phase 3 and on the commercial proposal.
6. **Shipping and post-sale:** labels, tracking, payout release, returns, disputes. Depends on phase 5 and on Melhor Envio's confirmation.
7. **Launch:** reviews, complete notifications, LGPD self-service, console metrics, production domains, homologations, manual test plan, release 1.0.0.

Future markets and cross-border sales come after launch: a regional gateway account or a second gateway, import-tax lines at checkout, per-market tax rules with effective dates, and the privacy and consumer rules of the target jurisdiction reviewed by a lawyer.

## 11. Proposed CLAUDE.md

The file is committed at the repository root in this pull request. It restates the working agreements of §3, adds the rule agreed in this session to follow market best practices, and points to the documents and commands Claude Code must use.
