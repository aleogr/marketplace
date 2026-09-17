# Marketplace Platform: Requirements

> **Status:** living document and source of truth for product requirements.
> Changes to this file must go through a pull request.
>
> **Conventions used in this document:**
> - Items without a marker are decisions made by the owner.
> - Items marked **(proposed)** are recommendations that still need the owner's confirmation during design.
> - Items listed under [Open topics](#29-open-topics-for-design) must be explored and decided during design.
> - Legal topics (terms of use, privacy, consumer law, marketplace regulations) must be reviewed by a qualified lawyer before launch.
>
> **Related documents:** `docs/research/competitive-analysis.md` (to be created).

---

## 1. Vision

- An **international platform that hosts multiple marketplaces**, accessed by people from many countries over the internet.
- Each marketplace serves one or more niches. The first two are **electronics** and **vehicles**. Many other niches will follow (see [Long-term vision](#28-long-term-vision)).
- There is a **single owner** of the platform and of every marketplace. This is **not a SaaS**: nobody other than the owner creates or administers marketplaces.
- Inside each marketplace, products are sold by **stores owned by the owner** and by **third-party stores**.
- **Brazil is the first market to operate**, but it is only one of the platform's markets. The design must not treat Brazil as the center of the system or as a special case embedded in code.

## 2. Scope

### 2.1 MVP

- Two marketplaces: electronics and vehicles.
- Sales and payments operate **in Brazil only**.
- Focus on the core end-to-end transaction: **a store lists a product, a buyer finds it, pays, and the store receives the payout**.
- Two fully working languages: **en-US** and **pt-BR**. "Fully working" refers to the platform's user interface; content written by stores (listings) is displayed in its original language in the MVP (see [section 6](#6-internationalization)).

### 2.2 Foundations that are expensive to retrofit

The following must be designed from the start and implemented in realistic phases, even if parts of them are delivered after the first release: auditing, internationalization, role-based access control, two-factor authentication, privacy compliance, CI/CD, and the security practices listed in [section 26](#26-infrastructure-and-cost) and [section 27](#27-development-workflow-and-quality).

### 2.3 Later phases (the design must not prevent them)

- Paid listings and featured placement.
- Custom domains per marketplace (depends on re-evaluating Cloud Run domain mapping before production; see [section 7](#7-tenancy-and-domains)).
- New niches and a generalist marketplace.
- Other markets and cross-border sales.
- Automatic translation of listings.
- Discount coupons (the data model must be ready from the start; see [section 13](#13-coupons-and-discounts)).
- Self-hosted product videos (the MVP is proposed to use external links only; see [section 8.4](#84-media)).
- Additional notification channels (web push, SMS, WhatsApp).
- Advanced recommendations.

## 3. Working agreements

- **Language:** Claude Code always talks to the owner in **Portuguese**. All versioned content is written in **English**: code, comments, commit messages, pull request descriptions and documentation. The only exception is translation files for other languages.
- **Branches and pull requests:** each topic is developed in its own branch and merged through a pull request, which is always opened by Claude Code.
- **Manual steps:** when the owner must act outside a Claude Code session (GCP console, GitHub, Cloudflare or any other service), instructions are given **one at a time**. The owner executes, reports the result, and only then receives the next instruction.
- **Confirmation before acting:** when the owner asks to see something before an action (commit, push, merge, configuration change), Claude Code shows it and waits for the owner's explicit confirmation before executing.
- **Secrets:** the repository is public. No secret may ever be committed.
- **Definition of done:** a feature is only done when all of its user-facing texts exist in **both en-US and pt-BR**, tests pass, and there is verification evidence (test output, screenshots or equivalent).
- **UI principles:** immediate visual feedback when an element is pressed; respect the operating system's reduced-motion preference; animations only when they serve a purpose.

## 4. Business model

- Buyers pay through a **checkout inside the platform**. The platform collects the payment and pays out to the stores (split payment).
- Platform revenue:
  - **Commission** on each sale.
  - **Paid listings / featured placement** (later phase).
- Buyer and seller **accounts are separate per marketplace**. Data from one marketplace must never be visible to buyers or sellers of another. Platform staff belong to the platform, not to a marketplace; their access to each marketplace's data is governed by roles and permissions (see [section 19](#19-administration-roles-and-permissions)).

## 5. Markets

- The system has an explicit concept of **market**, grouping country-specific rules such as currency, payment methods, taxes, consumer rules and shipping.
- **Brazil (MVP):**
  - Payment methods: **PIX** and **credit card with installments**.
  - Consumer right of withdrawal for online purchases.
- **Future markets and cross-border sales:** payments and payouts in other currencies, taxation and import rules, international shipping, and privacy and consumer laws of other jurisdictions (for example GDPR and European Union rules for online platforms). Phases and risks must be mapped; full solutions are not required now.

## 6. Internationalization

- The official and default language is **English (en-US)**.
- MVP languages: **en-US and pt-BR**, both fully working from the start. pt-BR exists from day one to validate the translation mechanism throughout development.
- New languages must be addable **without code changes**, only by adding translation files.
- **A URL with an explicit language always wins.** A visitor from Brazil who opens `/en-US/...` sees English.
- Visitors can **always** switch language manually, and that choice is remembered on later visits. **A remembered choice always wins over detection by country.**
- Automatic detection by country acts only when there is no remembered choice and the address contains no language (for example the home page), redirecting to the appropriate version: visitors accessing from **Brazil are redirected to pt-BR**. (The owner already implements the detection logic in another project.)
- Country detection uses an **IP geolocation database** (such as MaxMind GeoLite2), complying with its attribution requirement and its requirement to keep the data up to date.
- The **language is part of the URL**, for per-language SEO. Automatic detection must not prevent search engines from indexing pages in any language.
- **No hard-coded user-facing text**: everything goes through translation keys. Claude Code produces and maintains all translations.
- Monetary values are always stored **with a currency code, in minor units**.
- Dates are stored in **UTC** and formatted according to the user's locale.
- **Addresses must not assume any country's format.**
- Categories, attributes and institutional texts support translations.
- **Listing translation:**
  - Each listing records the language it was written in, and the data model supports listing translations.
  - MVP: listings are displayed in their original language.
  - Later phase: translate automatically when a listing is saved, store the translation, label it as automatically translated and keep the original accessible. Each translation records the provider and model that produced it.

## 7. Tenancy and domains

- The marketplace is identified by the **host of the incoming request**, from day one.
- Development domains:
  - Platform: `plataforma.lab.aleogr.dev`
  - Marketplaces: subdomains such as `marketplace1.plataforma.lab.aleogr.dev`
- DNS is managed in **Cloudflare**, using CNAME records to `ghs.googlehosted.com` in **"DNS only"** mode, with **Cloud Run's native domain mapping** (the same model the owner already uses in another project).
- Domain mapping does not support wildcards: **each marketplace requires its own domain mapping and DNS record**, and certificate issuance can take up to 24 hours. Creating a marketplace therefore includes an infrastructure step.
- Cloud Run domain mapping is a preview feature not recommended by Google for production. This must be re-evaluated before production traffic and custom domains.
- In the future, each marketplace will have its **own domain** (for example `marketplace1.com`).

### 7.1 Search engine indexing

- The lab host serves exactly the same pages the real domain will serve. If search engines index the lab first, the same content exists at two addresses and the lab is the one they already know. Redirecting afterwards is slow and lossy, and none of it is visible until months later.
- Deployments are **not indexable by default**. Indexing is enabled by explicit opt-in through the `INDEXABLE` environment variable, so that enabling it on launch day is a single command. The errors are not symmetric: a real deployment left non-indexable is noticed in days and fixed by flipping a variable; an indexed lab is noticed in months and costs a domain migration. The default makes the cheap mistake.
- When indexing is off, **every response the process writes** carries the header `X-Robots-Tag: noindex, nofollow`, applied by a middleware mounted once. A `<meta name="robots">` tag is not used for this: it would have to be added to every template and remembered in the next one, and it only reaches HTML, leaving preview images, the sitemap, JSON and any other response silent. Search engines read both the same way.
- **Crawling stays allowed in both modes.** `robots.txt` keeps `Allow: /` and never uses `Disallow: /`. `robots.txt` answers "may this be fetched"; noindex answers "may this be listed". Blocking crawling hides the noindex, and a page the crawler cannot fetch can still be listed from someone else's link, without a description, because the crawler was never allowed to look. To refuse indexing, crawling must be allowed. When indexing is off, `robots.txt` omits only the `Sitemap:` line, with a comment in its place saying why.
- **Link previews keep working in both modes:** Open Graph and Twitter tags and the preview image are served normally. That is exactly why crawling stays allowed.
- **An unreadable value refuses to start.** It is never silently treated as `false`: a typo such as `INDEXABLE=ture` would otherwise leave a real deployment invisible to all search engines with no error, no log and no screen, the only symptom being the absence of traffic. The start-up log records which of the two modes is in effect, on every start.
- The variable is **declared in the Terraform configuration of the Cloud Run service** with its explicit default value, even while that default changes nothing. A key that exists only as an absence is a key nobody finds on the day it matters.
- If any page states `<meta name="robots" content="index, follow">`, it contradicts the header. Conflicting directives resolve to the most restrictive one, so the refusal wins. Because this is a claim about other people's crawlers, it is covered by an automated test, not by a sentence.
- Verification is done in **both modes against the running server**, not only with unit tests: with the variable off, every response carries the header and `robots.txt` offers no sitemap; with it on, no response carries the header and the sitemap is back.

### 7.2 Page descriptions

- `<meta name="description">`, `og:description` and `twitter:description` come from **a single source**. If each came from a different field, the page would describe itself one way to search engines and another way to messaging apps, and the two would diverge without anyone noticing.
- Descriptions are cut at about **160 characters**, at the end of a sentence, never in the middle of a word. If the platform does not cut, the search engine cuts, and then the ellipsis is theirs.
- Length is counted in **characters (runes), not bytes**. In Go, `len()` and slicing operate on bytes; in Portuguese that cuts around 150 characters and can split an accented letter, leaving invalid UTF-8 in the description.
- Compliance is **measured across all pages in the sitemap**, not a sample, and the number of pages over the limit is reported.

## 8. Catalog

### 8.1 Categories and attributes

- Products are organized in a **hierarchical category tree**.
- **Attributes, variations and units of measure belong to categories, not to marketplaces.** A niche marketplace uses part of the category tree; a generalist marketplace uses many branches of it.
- Subcategories inherit attributes from their parent categories **(proposed)**.
- The administrator **creates attributes freely through the console, without code changes**, with varied types (text, number with unit, single choice, multiple choice, yes/no, among others).
- For each attribute, a category defines whether it is:
  - **required**;
  - **filterable**;
  - **comparable** (used in product comparison);
  - **variant-defining** (such as voltage, color or size) or **descriptive** (such as material or capacity).
- Units of measure are supported, including fractional sales for future niches (for example construction materials).

### 8.2 Products, variants and offers

- A **product with variants** is a single product with a single page, shared general photos and shared reviews.
- A **variant** is a combination of the variant-defining attributes. Examples:
  - A pan in 127V and 220V, each voltage in several colors.
  - A garment in several sizes, each size in several colors.
- Each variant has its own **SKU, stock, price** and, optionally, its own photos.
- A unique item (such as a vehicle) is simply a product with a single variant. It has no stock quantity, but it is still subject to **availability control** (available, reserved, sold), because it is reserved during checkout (see [section 10](#10-cart-and-checkout)).
- The model separates the **product** (the catalog item, for example a book identified by ISBN) from the **offer** (a store's price, condition and stock), so that several stores can sell the same product. How much of this the MVP implements is an open topic.
- **Physical products require weight and dimensions**, because shipping quotes depend on them. Vehicles are exempt.

### 8.3 Product comparison

- Buyers can **compare products within the same category**.
- Comparison uses the attributes the category marks as comparable. A category can make those attributes required so that comparison is always meaningful (for example number of doors, engine power and transmission type for cars).

### 8.4 Media

- Products accept **photos and videos**. **(proposed)** In the MVP, videos are **links to externally hosted videos** (such as YouTube). Self-hosted video is a later phase (see [section 2.3](#23-later-phases-the-design-must-not-prevent-them)).
- Files are stored in **Cloud Storage**, not in the database.
- Limits on file count and size are **configurable in the console**. In the MVP, the limit for videos is by **number of links**, also configurable in the console.
- Images are resized and converted to efficient formats.
- Videos are a cost risk (storage, egress and transcoding), which is why self-hosted video is deferred.

### 8.5 Listing moderation

- Listings from third-party stores are subject to moderation (wrong category, prohibited items).

## 9. Search and filters

- Filters are **complex and functional**, built on top of flexible attributes, and **vary by category**.
- Good performance with the **lowest possible operating cost**. The choice between PostgreSQL features and a dedicated search engine is an open topic.

## 10. Cart and checkout

- **Persistent cart**, including for visitors who are not signed in, **merged** into the account's cart on sign-in.
- **One cart per marketplace**, since accounts are separate per marketplace.
- A cart with items from several stores is split into **one shipment per store**, each with its own shipping cost, but paid with **a single payment**.
- **No stock reservation while items are in the cart.** Stock is reserved only during checkout, for a short period. This also applies to **unique items** such as vehicles, where the reservation is most critical: a unique item must never be sold to two buyers.
- Price, stock and availability are **revalidated at checkout**, and the buyer is clearly informed of any change.
- A **shipping estimate is shown early**, before the final checkout step.
- A **"save for later"** list is available.
- Abandoned-cart reminders by email are **marketing notifications** and are only sent to buyers who consented to that kind of communication (see [section 17](#17-notifications)).
- The checkout model for vehicles is an open topic.

## 11. Payments

- Brazil (MVP): **PIX** and **credit card with installments**.
- **Split payments** with a **sub-account per store inside the payment gateway**. Store identity verification (KYC) is performed through the gateway.
- **Saved cards:**
  - The platform **never stores the card number or the security code**.
  - The gateway stores the card and returns a token. The platform stores only the token, card brand, last four digits and expiry date.
  - This keeps the platform out of the heaviest PCI DSS scope.
  - Saved cards are bound to the gateway that issued the token.
- **(proposed)** Store payouts are held until delivery is confirmed.
- **Gateway migration:** switching gateways would require stores to register again with the new gateway, while the old gateway remains active for pending payouts, refunds and chargebacks. The data model must allow **two gateways to coexist** during a migration. This is a requirement on the data model, not on the initial implementation, which has a single gateway (see [section 25](#25-external-integrations)).

## 12. Shipping

- The system **calculates shipping costs** through an external provider (carrier API or shipping aggregator), behind a replaceable integration (see [section 25](#25-external-integrations)).
- Shipping is quoted **per store shipment**.
- Scope of label generation and tracking in the MVP is an open topic.
- Future needs: heavy and bulky items, regional delivery, store pickup, international shipping.

## 13. Coupons and discounts

Coupons are a later feature, but **the order model must support discount lines from the start**.

Coupon dimensions to support:

- **Type:** percentage, fixed amount, free shipping.
- **Scope:** whole platform, marketplace, store, category or product.
- **Conditions:** minimum order value, first purchase, validity period.
- **Limits:** total uses and uses per buyer.
- **Stacking:** whether a coupon can be combined with others.

Who funds a discount (platform or store) and whether commission is calculated before or after discounts are open topics, because they affect split payments.

## 14. Orders and post-sale

- Orders store a **snapshot** of the delivery address, prices and relevant product data at purchase time. Later edits to addresses or products must not change past orders.
- Returns and the Brazilian consumer right of withdrawal are supported.
- Dispute handling between buyers and stores.

## 15. Buyer–seller messaging

- Buyers can **contact stores through messages inside the platform**.
- **(proposed)** Phone numbers, email addresses and links are detected and masked, to discourage deals outside the platform (which bypass commission and buyer protection).
- Users can report abusive messages, subject to moderation.
- Message retention must balance privacy obligations with the need to keep evidence for disputes.

## 16. Reviews and reputation

- **Product and purchase reviews:**
  - Only buyers with a verified purchase can review.
  - Legitimate negative reviews are never hidden.
  - Moderation rules are transparent.
- **Store reviews:** buyers can review stores, and stores can reply publicly.
- **Buyer reviews:** **(proposed)** buyers are not rated publicly. Stores can report problems with a buyer internally, feeding the platform's fraud detection.
- Store reputation is derived from reviews, complaint rates and other signals.

## 17. Notifications

- The notification system is **event-driven** (for example: order paid, shipped, delivered; new message received).
- Channels:
  - MVP: **in-app** and **email**.
  - Later: web push, SMS and WhatsApp (the last two have per-message costs).
- Users choose which notifications they receive on each channel.
- **Transactional** notifications (about the user's own orders) can always be sent; **marketing** notifications (including abandoned-cart reminders) require consent.
- Notification templates follow the translation rules.

## 18. Accounts, authentication and privacy

### 18.1 User panel

Platform staff, sellers and buyers all have a user panel that allows:

- Changing email and password.
- Managing delivery addresses (buyers): multiple addresses, one of them marked as default.
- Managing saved cards (buyers): multiple cards (see [section 11](#11-payments)).
- **(proposed)** Viewing active sessions and devices, and signing them out remotely.
- Full privacy self-service (see [section 18.3](#183-privacy)).

### 18.2 Two-factor authentication

- Supported methods: **physical security keys** (such as YubiKey, via WebAuthn), **authenticator apps** (TOTP) and **email codes**.
- A secure **account recovery** mechanism.
- Which methods are mandatory for each user type is an open topic. The owner account must use 2FA.

### 18.3 Privacy

- **Full LGPD compliance**, including:
  - exporting the user's own data;
  - requesting deletion;
  - tracking the status of the deletion request.
- Because the platform is international, the general design must also consider other privacy laws (such as GDPR).
- **Consent management** for non-essential cookies and tracking, and for marketing communications.
- Reconciling full auditability with deletion requests (retention periods, anonymization, legal retention obligations) is an open topic.

## 19. Administration: roles and permissions

- The system is bootstrapped with an **owner (master) account**, which belongs to the platform owner.
- The owner account is created through a **secure first-run process**. Credentials are never hard-coded (the repository is public).
- The owner account **cannot be deleted** and cannot lose its permissions. It requires 2FA.
- **Role-based access control:**
  - The system defines **granular permissions** (for example: moderate listings, view orders, suspend stores).
  - The owner creates **roles** in the console by combining permissions.
  - Roles are assigned to staff members.
  - Roles can be scoped to specific marketplaces. This is how staff access to each marketplace's data is restricted, complementing the isolation between marketplaces defined in [section 4](#4-business-model).
- Management must be flexible and easy, since more staff members will join in the future.
- All role and permission changes are audited.

## 20. Seller trust and safety

- The console allows **temporarily suspending** and **permanently banning** stores.
- Legal basis and good practices:
  - Clear **terms of use** defining prohibited conduct and consequences (to be written or reviewed by a lawyer).
  - Every measure records its reason (audited).
  - The store is notified and has an appeal channel.
  - Data needed for legal obligations and to prevent re-registration is retained after a ban.
- **Layered prevention** (the preferred approach, with suspension as a last resort):
  - identity verification through the payment gateway;
  - lower limits for new stores;
  - payout hold until delivery confirmation (see [section 11](#11-payments));
  - reputation based on reviews and complaint rates;
  - automated alerts for suspicious patterns.
- Future markets: platform-specific regulations (for example the European Union's requirements for statements of reasons and notice before terminating a business user) must be reviewed.

## 21. Auditing

- The system is **highly auditable**. Every operation by platform staff, stores and buyers is traceable: **who** did it, **what** was done, **when**, **from where**, and the **state before and after** when applicable.
- Every parameter change is audited.
- **(proposed)** Audit records are append-only and tamper-evident.

## 22. Metrics, behavior tracking and recommendations

- The system collects data to feed a **rich administrative console with usage metrics**.
- **Customer behavior is recorded** (for example: product viewed, searched, added to cart, purchased) to learn what to offer each customer.
  - Signed-in users: events are recorded **server-side**, linked to the account.
  - Anonymous visitors: a cookie holds only a visitor identifier, subject to consent where required.
  - **(proposed)** First-party tracking only, without third-party trackers.
- **Recommendations** start with simple techniques (for example "customers who bought this also bought" and category affinity). Example: a customer who bought a pillow may be interested in a blanket or a bed sheet set. Machine learning is a later phase.
- Where to store and how to query metrics and events without burdening the primary database or creating high fixed costs is an open topic.

## 23. Parametrization

- The system is **configurable through the console wherever it makes sense**, so the administrator can change behavior without code changes.
- Parameters have defaults, validation and change history (audited).
- Not everything should become a parameter; the design must justify each one.
- Examples: media limits per product, limits for new stores, moderation rules, notification settings, and the translation provider in the future.

## 24. Architecture and technology

- **Backend:** Go.
- **User interface:** server-side rendering with **templ + HTMX**, and **Alpine.js** for richer interactions in dashboards (for example multi-photo upload, forms whose fields change by category, tables with filters).
- **A single repository and a single binary**, with static assets embedded via `go:embed`.
- **Database:** PostgreSQL on Cloud SQL.
- **Media storage:** Cloud Storage.
- **Application-level protections:** since there is no load balancer or web application firewall in front of the service, the binary itself applies **rate limiting** on sensitive endpoints (sign-in, sign-up, password recovery, checkout) and sets **security HTTP headers**.

## 25. External integrations

- External integrations sit **behind interfaces defined by the application domain** (ports and adapters):
  - payments;
  - email;
  - shipping;
  - IP geolocation;
  - translation.
- **All of them must allow replaceable providers**, but **only one provider is implemented initially** for each. For translation, the first provider will be **Google Cloud Translation**.
- Contracts reflect the platform's needs, **not a specific provider's API**.
- Every external identifier (payment IDs, store sub-account IDs, tracking codes, card tokens) is **stored together with the provider that issued it**.
- Webhooks are received on provider-specific endpoints and translated into internal events.
- Tests use **fake implementations** of these interfaces, never real services.
- Replacing the payment gateway is a migration project, not just a new adapter (see [section 11](#11-payments)).

## 26. Infrastructure and cost

- **Google Cloud Platform**, region **us-central1**.
- **Cloud Run**, scaling to zero.
- **Cloud SQL** (PostgreSQL) and **Cloud Storage**.
- **No Kubernetes and no load balancer.**
- **Infrastructure as code with Terraform.** Environment variables of the Cloud Run service are declared there with explicit values (see [section 7.1](#71-search-engine-indexing)).
- **Least privilege:** separate service accounts for Cloud Run, Cloud SQL and Cloud Storage, each with only the permissions it needs. Buckets are private by default; public access is granted only to what must be public (such as product images).
- **Backups and recovery:** automatic Cloud SQL backups with a defined retention period, and periodic restore tests. Media in Cloud Storage is covered by a defined retention and recovery approach.
- **Strong premise: the lowest possible operating cost**, avoiding recurring fees until the platform generates revenue. The design must identify fixed costs and keep them minimal.

## 27. Development workflow and quality

- **CI** on every pull request: tests, lint, build, secret detection, **static security analysis** of the Go code and **vulnerability scanning** of dependencies (Go modules and container base image).
- Dependency updates are proposed automatically (for example by Dependabot) and go through the same CI.
- **CD**: deployment to Cloud Run after merge.
- **(proposed)** GitHub authenticates to GCP without stored keys (Workload Identity Federation).
- Environments: lab now; production in the future.
- **End-to-end tests** of real flows (sign-up, search, checkout) run in CI. Language and runner are open topics.
- Claude Code works exclusively through **Claude Code on the web**; the owner has no local development environment.
- The Claude Code cloud environment already has **Playwright (Python) and Chromium** installed, and the repository includes the **webapp-testing** skill for in-session verification of screens. The environment setup is documented in `docs/`.

## 28. Long-term vision

- **Many niches**, not only those mentioned here. Examples and their needs:
  - **Clothing and footwear:** variants (size, color) with stock and price per combination.
  - **Books:** the same product (ISBN) sold by several stores, new or used, at different prices.
  - **Construction materials:** varied units of measure and fractional sales, heavy and bulky items, regional delivery and store pickup.
  - **Vehicles** (already in the MVP) are the opposite extreme: each listing is a unique item without stock.
- **A generalist marketplace**, in the style of AliExpress, with products from every category at once.
- **International operation:** buyers and stores from any country, with each market's payment methods, currencies, taxes and logistics, plus cross-border sales.

## 29. Open topics for design

- Product model serving every niche and the generalist marketplace, without unnecessary complexity in the MVP and without requiring a redesign later (including how much of the product/offer separation the MVP implements).
- Indexing and querying complex filters over flexible attributes at low cost (PostgreSQL features versus a dedicated search engine).
- Modeling markets so that Brazil is the first market, not a special case.
- Checkout model for vehicles: full payment, deposit/reservation or another model, considering high values, fraud prevention and document transfer.
- Whether the Brazilian consumer right of withdrawal applies to vehicles, and its impact on the checkout model and on the payout hold (to be validated with a lawyer).
- Card installments: who pays the interest, impact on store payouts and interaction with split payments.
- Payment gateway selection for split payments, sub-accounts, PIX and installments, considering future markets and currencies.
- Shipping scope in the MVP: quotes only, or labels and tracking as well; provider selection.
- Coupon funding (platform or store) and the commission base (before or after discounts).
- How to enforce isolation between marketplaces (the rule is decided; see [section 4](#4-business-model)) and authorization between stores.
- Auditing versus privacy: retention periods, anonymization and legal retention obligations.
- Security incident response: what to do in case of a data breach, including the notification duties and deadlines under LGPD and, for future markets, other privacy laws (to be validated with a lawyer).
- Storage and querying of metrics and behavior events without high fixed costs.
- Which behaviors become console parameters.
- Mandatory 2FA methods per user type, and recovery flows.
- Marketplace creation flow, including the infrastructure step (domain mapping and DNS).
- Re-evaluation of Cloud Run domain mapping (a preview feature) before production traffic and before adopting custom domains per marketplace (see [section 7](#7-tenancy-and-domains)).
- How to keep the IP geolocation database up to date (using such a database is decided; see [section 6](#6-internationalization)): embedded in the binary or downloaded at startup; and how to read the visitor's IP correctly on Cloud Run.
- Exact URL format for the language (the language being part of the URL is decided; see [section 6](#6-internationalization)), compatible with automatic detection and SEO.
- Currencies and price conversion.
- Phases and risks for future markets and cross-border sales.
- CI/CD pipeline structure, keyless GitHub-to-GCP authentication, secret detection, security scanning tools and environments.
- Backup retention periods, restore test frequency and recovery targets.
- Language and execution of end-to-end tests in CI.
- How listing moderation for third-party stores works (moderation itself is decided; see [section 8.5](#85-listing-moderation)).
- Messaging rules: masking contact information, moderation and retention.
- How to implement rich dashboard interactions with HTMX + Alpine.js (the stack is decided; see [section 24](#24-architecture-and-technology)).
- Media limits, image processing and the video strategy.
- Fixed costs and how to keep them minimal.
- MVP boundary: what is in, what is out, and the order of implementation phases.
