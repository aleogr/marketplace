# Marketplace Platform: Requirements

> **Status:** living document and source of truth for product requirements.
> Changes to this file must go through a pull request.
>
> **Conventions used in this document:**
> - Items without a marker are decisions made by the owner.
> - Items listed under [Open topics](#29-open-topics-for-design) still need an external input (a lawyer, a commercial proposal, a spike) before they can be decided; topics closed during design are recorded in `docs/design.md`.
> - Legal topics (terms of use, privacy, consumer law, marketplace regulations) must be reviewed by a qualified lawyer before launch.
>
> **Related documents:** `docs/design.md` (how the product is built: architecture, data model, design decisions and phases), `docs/research/competitive-analysis.md` (competitors), `docs/research/payment-providers.md` (payment gateway candidates), `docs/research/shipping-providers.md` (shipping provider candidates, with an appendix on transactional e-mail). Decisions below that came from those analyses cite them.

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

- Paid listings and featured placement for goods marketplaces (the vehicles marketplace may launch with them; see [section 4](#4-business-model)).
- Store subscription plans as a third revenue stream (Mercado Livre, Amazon and eBay sell them).
- Vehicle trust services around a listing: vehicle history reports, inspection badges with an expiry date, dealer warranties, financing partners with pre-approved credit leads. The listing model must be able to carry badges with an expiry date from the start.
- Custom domains per marketplace (depends on re-evaluating Cloud Run domain mapping before production; see [section 7](#7-tenancy-and-domains)).
- New niches and a generalist marketplace.
- Other markets and cross-border sales.
- Automatic translation of listings.
- Discount coupons (the data model must be ready from the start; see [section 13](#13-coupons-and-discounts)).
- Self-hosted product videos (the MVP uses external links only; see [section 8.4](#84-media)).
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
- Platform revenue is defined **per marketplace**:
  - **Commission** on each sale, the model for goods marketplaces.
  - **Paid listings / featured placement.** A later phase for goods; the **launch model for the vehicles marketplace**, which charges no commission on the sale or on the reservation (commission on the reservation is a parameter, zero by default), since no competitor charges a commission on a vehicle sale (they sell listing tiers with exposure levels and durations, mandatory plans or per-lead pricing to dealers; see `docs/research/competitive-analysis.md`, section 8). The data model must not assume a cost-per-click model: fixed-price highlights with a duration, cost-per-click auctions and cost-per-sale percentages all exist in the market.
- **Fee schedule:** fees are a versioned schedule per marketplace with effective dates, supporting a percentage by category, a fixed fee per item or per order, a minimum commission and price bands. Competitors combine all of these and change them often. The schedule is a console parameter (see [section 23](#23-parametrization)).
- **Commission base:** the base is the item amount actually paid by the buyer to the store, **after store-funded discounts** (store coupons, a PIX discount the store chose to give) and **before platform-funded discounts**, which do not reduce the base; the shipping charged to the buyer is outside the base, because it is a pass-through cost and the store is the shipper. On a partial cancellation the commission of the cancelled item is returned proportionally. The commission of an order is recorded as an explicit split line at payment time, never derived afterwards, and the platform's own stores also have a sub-account at the gateway.
- **Fee evaluation:** provider fees are compared on the expected ticket distribution of each marketplace, not on a single average; a flat PIX fee and a percentage fee behave in opposite ways for vehicle deposits and for cheap accessories.
- The vehicles marketplace distinguishes **dealers** (professional sellers) from private sellers, with their own limits and pricing; every competitor treats dealers as a distinct seller type.
- Buyer and seller **accounts are separate per marketplace**. Data from one marketplace must never be visible to buyers or sellers of another. Platform staff belong to the platform, not to a marketplace; their access to each marketplace's data is governed by roles and permissions (see [section 19](#19-administration-roles-and-permissions)).

## 5. Markets

- The system has an explicit concept of **market**, grouping country-specific rules such as currency, payment methods, taxes, consumer rules and shipping.
- **Brazil (MVP):**
  - Payment methods: **PIX** and **credit card with installments**.
  - Consumer right of withdrawal for online purchases.
- **Future markets and cross-border sales:** payments and payouts in other currencies, taxation and import rules, international shipping, and privacy and consumer laws of other jurisdictions (for example GDPR and European Union rules for online platforms). Phases and risks must be mapped; full solutions are not required now. Two facts from the research shape the design: cross-border buying in Brazil is implemented by charging the buyer's estimated import taxes at checkout, so the order model needs an import-tax line and the rule that the buyer is the importer; and tax rules change (the Brazilian rates changed in May 2026 while competitors' help pages still showed the old ones), so tax rules are data with effective dates, never code. No payment provider offers a Brazilian platform cross-border payouts self-serve; the first foreign market will most likely mean a regional account or a second gateway (see [section 11](#11-payments)).

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
- Subcategories inherit attributes from their parent categories. A subcategory can add attributes and make an inherited attribute required, but cannot remove it. Moving a category changes the effective attributes of the products beneath it, and the design must handle that case.
- The administrator **creates attributes freely through the console, without code changes**, with varied types (text, number with unit, single choice, multiple choice, yes/no, among others).
- For each attribute, a category defines whether it is:
  - **required**;
  - **filterable**;
  - **comparable** (used in product comparison);
  - **variant-defining** (such as voltage, color or size) or **descriptive** (such as material or capacity).
- Units of measure are supported, including fractional sales for future niches (for example construction materials).
- **Vehicle category attributes** observed as mandatory or as filters at every vehicle competitor: licence plate (mandatory, used to pre-fill vehicle data), mileage, fuel, transmission, body type, colour, doors, documentation status, plate ending, around thirty equipment items, and a price reference (FIPE table). The category design includes plate lookup, a FIPE reference and a price-anomaly check: competitors pause or reject listings far from the market price.

### 8.2 Products, variants and offers

- A **product with variants** is a single product with a single page, shared general photos and shared reviews.
- A **variant** is a combination of the variant-defining attributes. Examples:
  - A pan in 127V and 220V, each voltage in several colors.
  - A garment in several sizes, each size in several colors.
- Each variant has its own **SKU, stock, price** and, optionally, its own photos.
- A unique item (such as a vehicle) is simply a product with a single variant. It has no stock quantity, but it is still subject to **availability control** (available, reserved, sold), because it is reserved during checkout (see [section 10](#10-cart-and-checkout)).
- The model separates the **product** (the catalog item, for example a book identified by ISBN) from the **offer** (a store's price, condition and stock), so that several stores can sell the same product. The electronics marketplace implements the separation from the MVP, matching offers to catalog products by GTIN/EAN, with a **Buy Box** that picks the featured offer; the vehicles marketplace keeps one listing per store. The Buy Box criteria (price, interest-free installments, shipping, store reputation, stock, as used by Mercado Livre, Amazon, Magalu and KaBuM) are an open design item.
- Listings can carry **badges with an expiry date** (for example an inspection badge valid for 120 days).
- **Physical products require weight and dimensions**, because shipping quotes depend on them. Vehicles are exempt. Carriers bill the greater of the physical weight and the volumetric weight (length × width × height / 6000 at Mercado Livre and Amazon), so the platform keeps the volumetric weight as computed by the shipping provider (factors differ by carrier: 6000 for most, 3333 for Jadlog inter-state at Melhor Envio) and never recomputes it, and a listing's measurements can be corrected after a carrier re-measures a parcel. Listing validation enforces the carriers' dimensional minimums (Correios 11 × 6 × 0,4 cm; Loggi 10 cm width and 15 cm length).
- **Fiscal document per shipment:** since 6 April 2026 every parcel without an NF-e needs an electronic content declaration (DC-e), which the shipping providers file with SEFAZ from the product data, and companies that are ICMS taxpayers cannot use it. The store profile carries an **ICMS taxpayer** flag; a shipment carries either the NF-e key or the product data for the declaration, and the store attaches the NF-e before the label is generated. Under a content declaration carriers give lower indemnity caps (about R$ 1.000 to 1.500) and sometimes no damage cover, so high-value electronics ship with an NF-e and declared value; individual sellers are warned of the difference.

### 8.3 Product comparison

- Buyers can **compare products within the same category**, three to four at a time. No Brazilian generalist offers attribute comparison of arbitrary listings; it is a differentiator.
- Comparison uses the attributes the category marks as comparable. A category can make those attributes required so that comparison is always meaningful (for example number of doors, engine power and transmission type for cars).

### 8.4 Media

- Products accept **photos and videos**. In the MVP, videos are **links to externally hosted videos** from a closed list of allowed providers (such as YouTube and Vimeo). The player is loaded only after the visitor clicks the thumbnail, using the provider's cookie-free embed where available, so that no third-party cookie is set before consent. The link is stored as provider plus external identifier (see [section 25](#25-external-integrations)). Self-hosted video is a later phase (see [section 2.3](#23-later-phases-the-design-must-not-prevent-them)).
- Files are stored in **Cloud Storage**, not in the database.
- Limits on file count and size are **configurable in the console**. In the MVP, the limit for videos is by **number of links**, also configurable in the console. Reference values in the market: 15 photos with a minimum of 800×600 for vehicles at Mercado Livre, 20 photos for vehicles and 6 for goods at OLX, 500×500 on a white background at KaBuM.
- Images are resized and converted to efficient formats.
- Videos are a cost risk (storage, egress and transcoding), which is why self-hosted video is deferred.

### 8.5 Listing moderation

- Listings from third-party stores are subject to moderation (wrong category, prohibited items).
- Moderation happens **before publication**, as the direct competitors of both niches do (KaBuM approves products before offers go live, Webmotors analyses every ad in seconds to one business day, OLX within 24 hours), with **automatic pre-checks** (category fit, contact data in the text, price anomaly) and a manual queue. Contact data (phone numbers, e-mail addresses, links) is detected in listing text, questions and reviews, not only in messages (see [section 15](#15-buyerseller-messaging)).

## 9. Search and filters

- Filters are **complex and functional**, built on top of flexible attributes, and **vary by category**.
- Standard sort options: relevance, price ascending and descending, best sellers, best rated and newest. The search design accounts for the aggregates they need (sales count, rating average).
- Good performance with the **lowest possible operating cost**: search runs on **PostgreSQL only** (JSONB attributes with GIN indexes, a full-text vector per language, trigram matching for typos and materialized aggregates for sorting), behind a search port so that a dedicated engine can be added later without touching the modules that call it (see `docs/design.md`, section 2.7).

## 10. Cart and checkout

- **Persistent cart**, including for visitors who are not signed in, **merged** into the account's cart on sign-in.
- **One cart per marketplace**, since accounts are separate per marketplace.
- A cart with items from several stores is split into **one shipment per store**, each with its own shipping cost, but paid with **a single payment**.
- **No stock reservation while items are in the cart.** Stock is reserved only during checkout, for a short period. This also applies to **unique items** such as vehicles, where the reservation is most critical: a unique item must never be sold to two buyers.
- Price, stock and availability are **revalidated at checkout**, and the buyer is clearly informed of any change.
- A **shipping estimate is shown early**, before the final checkout step.
- A **"save for later"** list is available.
- The order model reserves a **pickup point** delivery type (agencies, partner shops, lockers), even though pickup is a later phase.
- Abandoned-cart reminders by email are **marketing notifications** and are only sent to buyers who consented to that kind of communication (see [section 17](#17-notifications)).
- **Vehicles checkout:** no Brazilian competitor processes the price of a vehicle; the models are a reservation held by the platform until delivery (Mercado Livre), a deposit with the balance off-platform (eBay) and bank financing embedded in the listing (Webmotors, OLX). In the MVP the buyer pays a **reservation or deposit through the platform**, held until both parties confirm the handover, and the balance is settled outside the platform; full in-platform payment is a later option. The reservation is a **captured payment** (PIX or card) held in escrow or in the platform balance, not a card pre-authorization: providers' pre-authorization windows range from 3 to 29 days. The reservation interacts with the rule that a unique item is reserved during checkout. Online reservation is **optional per listing**, enabled by the store as a trust feature; listings without it are contact-only. **Reservation rules** (all marketplace parameters): the amount is a percentage of the listed price with a floor and a cap (defaults 2%, R$ 500 and R$ 5.000; the cap must stay below the maximum transaction amount the gateway allows); both parties have N days (default 7) to confirm the handover; the buyer's withdrawal is fully refunded while the right of withdrawal for vehicles is not validated by a lawyer (forfeiture is a parameter, off by default); the store's withdrawal is fully refunded and marks a cancellation on its reputation; expiry without confirmations refunds automatically; confirmation by only one party opens a dispute.
- A cart with items from several stores requires a gateway that splits **one payment among several recipients**; gateways limited to single-seller carts (PagBank, as published) are excluded (see `docs/research/payment-providers.md`).

## 11. Payments

- Brazil (MVP): **PIX** and **credit card with installments**.
- **Split payments** with a **sub-account per store inside the payment gateway**, including the platform's own stores. Store identity verification (KYC) is performed through the gateway; whether the gateway runs the verification itself is a **selection criterion**, because some providers leave the analysis to the platform (Iugu) or require the store to finish it in the provider's app (PagBank).
- **Store onboarding** is a state machine (created, documents pending, under review at the provider, active, refused, suspended) fed by the gateway's events. The platform stores the verification **status and the provider's reference, never the documents**, and prefers hosted or link-based document collection over API-only onboarding, which would put sensitive documents on the platform.
- **Saved cards:**
  - The platform **never stores the card number or the security code**, and the card number **never transits the platform's servers**: tokenization happens in the browser (PCI SAQ A). A gateway whose documented integration tokenizes on the server side keeps the platform in PCI scope and fails this criterion.
  - The gateway stores the card and returns a token. The platform stores only the token, card brand, last four digits and expiry date.
  - This keeps the platform out of the heaviest PCI DSS scope.
  - Saved cards are bound to the gateway that issued the token.
- **Store payouts are held until delivery is confirmed.** Every competitor does this (48 hours after confirmation at OLX, delivery plus 7 days at Shopee and Amazon, longer for stores without reputation), and every candidate gateway supports it through a named escrow (Asaas, PagBank), disabled automatic payouts with manual transfers (Pagar.me) or a platform-held balance (Stripe, 90-day limit). The order records **who holds the money** (store sub-account under escrow or platform balance) and the **release event**, so the hold survives a gateway change. The hold length is a parameter driven by store reputation, shipping mode (platform label with tracking or own shipping) and delivery confirmation, with a fallback deadline that fits within the provider's maximum (365 days at PagBank, 90 at Stripe). Delivery confirmation comes from the shipping provider's tracking (see [section 12](#12-shipping)).
- The payment institution may **block balances or impose reserves unilaterally** (every contract reviewed allows it). Store-facing texts about payout timing say so, and provider-side blocks received by webhook are shown on the store's payout screen.
- **Installments:** the platform owns the installment plan (number of installments, interest-free limit per store or listing, resulting amounts) and treats each provider's cost table as configuration, so the checkout shows the same plans whatever the gateway. Interest-free installments are a **seller-funded, opt-in** feature with an explicit cost (Mercado Livre, Amazon, OLX and Magalu all price it that way); otherwise the buyer pays the interest. Providers differ on who computes buyer interest (PagBank computes it; Asaas and Adyen expect the platform to send the amounts).
- **Chargebacks** are the store's cost by default, softened by the gateway's protection when the platform's tracked shipping label was used; the allocation rule is written in the terms of use and applied to the split (per-recipient liability flags exist at Pagar.me and PagBank).
- **Gateway migration:** switching gateways would require stores to register again with the new gateway, while the old gateway remains active for pending payouts, refunds and chargebacks. The data model must allow **two gateways to coexist** during a migration. This is a requirement on the data model, not on the initial implementation, which has a single gateway (see [section 25](#25-external-integrations)).

## 12. Shipping

- The system **calculates shipping costs** through an external provider (carrier API or shipping aggregator), behind a replaceable integration (see [section 25](#25-external-integrations)).
- Shipping is quoted **per store shipment**. Quotes are per package: providers either pack the items into a box for the platform (Melhor Envio, SuperFrete) or accept pre-packed volumes, and multi-volume shipments exist only on some carriers. The shipment model supports several volumes and records the box returned by the quote, which is reused when the label is bought.
- **The platform integrates the store's shipping account; it is not the shipper.** The market's model for many third-party stores is one provider account per store, linked to the platform by OAuth (Melhor Envio, SuperFrete, Loggi) or created through a partner API (Frenet), with the store's own wallet paying for labels; a single platform account is contractually restricted at the aggregators (see `docs/research/shipping-providers.md`, section 7). The platform quotes with the store's token and drives the store's account. "Platform as shipper under its own carrier contract" (a direct Correios contract with authorised users, with the platform liable and a minimum consumption) is a later option.
- **Store onboarding includes a shipping step:** authorise the provider, verify documents (Melhor Envio allows 3 simultaneous labels until verified, SuperFrete 5), fund the wallet. A store cannot publish physical products, or the checkout treats it as unable to ship, until it can generate a label.
- The MVP includes **label generation and tracking** through the shipping provider, not quotes only: platform-generated labels with tracking are the norm at every Brazilian marketplace, and the tracking event "delivered" is what releases the store payout (see [section 11](#11-payments)). Polling is the fallback for providers without webhooks (Correios below its top tiers) and a scheduled reconciliation catches shipments whose events were missed.
- **The shipping cost is not final at checkout:** carriers re-measure parcels and debit the difference from the store's wallet later. The order stores the quoted service, price and deadline, and has a post-delivery adjustment line for the difference, absorbed by the store by default.
- **Deadlines** returned by providers are "days" without stating business days (only Correios says business days). The checkout shows a date range computed by the platform from the provider's range, never calendar arithmetic on a single number.
- **Labels expire** (10 days at SuperFrete, 15 at Correios, 20 at Melhor Envio) and can be cancelled before posting with a refund to the store's wallet. The order workflow has a "label expired" state with a re-purchase path, and the store's dispatch deadline (a reputation metric) is shorter than the label expiry.
- The MVP assumes **drop-off** at agencies and partner points, which is free and universal; pickup at the store's address is a paid or contract feature (R$ 49,90 under 10 parcels at Loggi; contract-only at Correios) and stays a later phase.
- Provider selection is an open topic; `docs/research/shipping-providers.md` shortlists Melhor Envio first and SuperFrete as the second adapter behind the same interface.
- **Free shipping rules** are threshold-based and subsidised in the market (R$ 19 and R$ 79 at Mercado Livre and Amazon, R$ 99 at Magalu). "Free shipping above a threshold" is a rule at marketplace or store level with a defined payer (store, platform or shared), separate from coupons.
- Future needs: heavy and bulky items, regional delivery, store pickup, international shipping.

## 13. Coupons and discounts

Coupons are a later feature, but **the order model must support discount lines from the start**.

Coupon dimensions to support:

- **Type:** percentage, fixed amount, free shipping.
- **Scope:** whole platform, marketplace, store, category or product.
- **Conditions:** minimum order value, first purchase, validity period, payment method (a "PIX discount" is a common promotion; the commission base rule must say whether it applies before or after that discount).
- **Caps:** a maximum discount amount per coupon (free-shipping coupons at Shopee cover up to R$ 20).
- **Limits:** total uses and uses per buyer.
- **Stacking:** whether a coupon can be combined with others.

A discount is funded by the **platform or by the store**, recorded on each discount line, because it affects split payments: store-funded discounts reduce the commission base and platform-funded discounts do not (see [section 4](#4-business-model)).

## 14. Orders and post-sale

- Orders store a **snapshot** of the delivery address, prices and relevant product data at purchase time. Later edits to addresses or products must not change past orders.
- Returns and the Brazilian consumer right of withdrawal are supported. The rules define **who pays return shipping by reason** (the store only when the item is different or defective, as at Mercado Livre and Shopee) and whether the **commission is refunded** on a cancellation or return, and whether a cancellation fee exists (Mercado Livre refunds its fee only on cancellation; KaBuM charges 6,5% on refunded orders).
- **Processing fees are not returned on refunds** at any gateway reviewed. The terms decide who absorbs the non-refunded fee on a cancellation or return: the store, the platform or the buyer through a deduction.
- **Consumer returns** use the Brazilian model: a Correios "reverse" posting code presented at a counter, not a printed label, requested by the store (or by the platform on its behalf) through the shipping provider and paid from the store's wallet; the return travels with a content declaration even when the outbound had an invoice. The flow: the reverse is requested, the buyer receives the code and the declaration, and the tracking of the reverse feeds the refund.
- **Lost or damaged parcels** are settled between the store and the carrier: indemnity is capped by document type (about R$ 25 automatic at Correios; R$ 1.000 to 1.500 under a content declaration; higher with an NF-e and declared value) and paid to the store's wallet by the carrier, not by the platform. The dispute rules say what the buyer receives meanwhile.
- Dispute handling between buyers and stores.
- **Chargeback evidence:** the order keeps an evidence pack (the full tracking history received by webhook, not only the final status, since proof of delivery with a signature exists only at Loggi through the API and at Correios as a paid add-on; delivery proof; invoice; buyer messages) that can be exported as a single PDF, because gateways ask for evidence through their APIs under per-case deadlines and page limits (10 calendar days at Mercado Pago; single PDF of up to 18 pages at PagBank). A dispute deadline is a notification event.

## 15. Buyer–seller messaging

- Buyers can **contact stores through messages inside the platform**.
- Phone numbers, email addresses and links are detected and masked, to discourage deals outside the platform (which bypass commission and buyer protection). The same detection applies to listing text, questions and reviews (Webmotors blocks contact data in descriptions, Mercado Livre demotes such listings, Shopee bans off-platform contact). Competitors deliberately allow public phone and WhatsApp for vehicles because the transaction happens off-platform. **Contact-data detection is a per-marketplace flag**: on for goods marketplaces, off for vehicles, where the store chooses whether to show its phone and WhatsApp on the listing, because masking is porous (a number in a photo) and the vehicles revenue comes from listings, not from the transaction.
- Listings have a **public question-and-answer section** in the MVP (as at Mercado Livre, Amazon and Magalu), reusing the messaging mechanism: the store answers publicly, questions go through the same detection and moderation as messages, and unanswered questions expire from the public view after a parameterized period.
- Users can report abusive messages, subject to moderation.
- Message retention must balance privacy obligations with the need to keep evidence for disputes.

## 16. Reviews and reputation

- **Product and purchase reviews:**
  - Only buyers with a verified purchase can review, within a review window after delivery (Shopee allows 30 days).
  - Reviews accept photos and videos.
  - Reviews attach to the product and are shared by all offers and variants.
  - Reviews are moderated before publication with explicit rejection reasons (contact data, links, unrelated content), as at Mercado Livre, Amazon and Magalu.
  - Legitimate negative reviews are never hidden.
  - Moderation rules are transparent.
  - Rewards for reviews (coupons at Mercado Livre, coins at Shopee) exist in the market and are noted, not adopted.
- **Store reviews:** buyers can review stores, and stores can reply publicly.
- **Buyer reviews:** buyers are not rated publicly; no competitor does it (OLX hides buyer ratings, eBay allows only positive feedback). Stores can report problems with a buyer internally, feeding the platform's fraud detection.
- **Store reputation** is quantitative, derived from defined metrics over defined windows: complaint rate, seller-cancellation rate, late-dispatch rate, valid-tracking rate and response time, measured over 60 or 365 days as at Mercado Livre. Published thresholds in the market cluster around complaints ≤ 2%, seller cancellations ≤ 1,5–2,5%, late dispatch ≤ 3–4%, tracking ≥ 95% and replies within 2 business days. Reputation gates concrete outcomes (payout hold length, Buy Box eligibility, exposure, graded sanctions) and is shown only after a minimum number of completed sales (10 at Mercado Livre and KaBuM). Metrics, windows, thresholds and consequences are console parameters (see [section 23](#23-parametrization)).

## 17. Notifications

- The notification system is **event-driven** (for example: order paid, shipped, delivered; new message received).
- Channels:
  - MVP: **in-app** and **email**.
  - Later: web push, SMS and WhatsApp (the last two have per-message costs).
- Users choose which notifications they receive on each channel.
- **Transactional** notifications (about the user's own orders) can always be sent; **marketing** notifications (including abandoned-cart reminders) require consent.
- Notification templates follow the translation rules.
- Shipping notifications link to the provider's public tracking page instead of duplicating scan events; the platform's own e-mails carry the lifecycle states it receives (posted, delivered, returned).

## 18. Accounts, authentication and privacy

### 18.1 User panel

Platform staff, sellers and buyers all have a user panel that allows:

- Changing email and password.
- Managing delivery addresses (buyers): multiple addresses, one of them marked as default.
- Managing saved cards (buyers): multiple cards (see [section 11](#11-payments)).
- Viewing active sessions and devices, and signing them out remotely. Sessions are stored server-side and revocable from the first release, and a password change ends all other sessions of the account; the screen itself may be delivered after the first release.
- Full privacy self-service (see [section 18.3](#183-privacy)).

### 18.2 Two-factor authentication

- Supported methods: **physical security keys** (such as YubiKey, via WebAuthn), **authenticator apps** (TOTP) and **email codes**.
- A secure **account recovery** mechanism.
- **Mandatory methods per user type:** the owner and staff must use an authenticator app or a security key (e-mail codes are not accepted as a second factor for console users, because e-mail is their recovery channel); stores are asked for a second factor on sensitive actions (bank or payout account changes, e-mail and password changes, bulk label generation, acceptance of terms), with any method and the e-mail code as the default for a new store; buyers use 2FA optionally, with a step-up by e-mail code when adding a card or changing the e-mail address.
- **Recovery:** one-time recovery codes are generated when an authenticator app or a security key is enrolled; a staff account is recovered only by the owner or another administrator resetting its factor, with an audit record; stores and buyers recover through e-mail with a security delay (default 24 hours, a parameter), a notification on every channel and revocation of all sessions.

### 18.3 Privacy

- **Full LGPD compliance**, including:
  - exporting the user's own data;
  - requesting deletion;
  - tracking the status of the deletion request.
- Because the platform is international, the general design must also consider other privacy laws (such as GDPR).
- **Consent management** for non-essential cookies and tracking, and for marketing communications.
- Deletion requests are reconciled with auditing through per-user encryption keys (see [section 21](#21-auditing)). Retention periods and legal retention obligations are an open topic.

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
  - The store is notified and has an appeal channel with a response deadline that is a parameter (Shopee: appeal within 30 days, answer in 5 days). Sanctions are graded by score: pause a listing, limit the listing quota, suspend temporarily (15 days at KaBuM), ban.
  - Data needed for legal obligations and to prevent re-registration is retained after a ban.
- **Layered prevention** (the preferred approach, with suspension as a last resort):
  - identity verification through the payment gateway (biometric liveness is required by Pagar.me, Asaas, Stripe and PagBank): **every store in every marketplace completes the gateway onboarding**, including stores that only buy listings, and the gateway's verification state is shown as a **"verified identity" badge**; a dedicated verification provider remains pluggable behind the same interface but is not needed in the MVP;
  - lower limits for new stores: "new store" is a state that ends after a number of completed sales without complaints, with a longer payout hold, a cap on simultaneous listings and exclusion from the Buy Box as parameters (the instruments competitors actually use); these limits sit on top of the gateway's own ramps (recipients cannot withdraw until active at Pagar.me; 10 sub-accounts of R$ 2.000 for 60 days at Asaas), and the launch plan schedules the provider's homologation;
  - payout hold until delivery confirmation (see [section 11](#11-payments));
  - reputation based on reviews and complaint rates;
  - automated alerts for suspicious patterns.
- Future markets: platform-specific regulations (for example the European Union's requirements for statements of reasons and notice before terminating a business user) must be reviewed.

## 21. Auditing

- The system is **highly auditable**. Every operation by platform staff, stores and buyers is traceable: **who** did it, **what** was done, **when**, **from where**, and the **state before and after** when applicable.
- Every parameter change is audited.
- KYC results from the gateway are stored as status and provider reference only (approved, refused, redo), never as documents.
- Audit records are **append-only** (no updates or deletions, enforced by database permissions) and **tamper-evident** (each record carries the hash of the previous one, and the chain is verified periodically).
- Audit records reference people by **internal identifiers**, never by name, email or document number. Fields that inherently carry personal data (such as the state before and after, or the origin IP address) are stored **encrypted with a per-user key** kept outside the log. A deletion request is fulfilled by **destroying that key**: the record and the hash chain stay intact, and the content becomes unrecoverable. Keys are retained while a legal retention obligation applies (see [section 18.3](#183-privacy)).

## 22. Metrics, behavior tracking and recommendations

- The system collects data to feed a **rich administrative console with usage metrics**.
- **Customer behavior is recorded** (for example: product viewed, searched, added to cart, purchased) to learn what to offer each customer.
  - Signed-in users: events are recorded **server-side**, linked to the account.
  - Anonymous visitors: a cookie holds only a visitor identifier, subject to consent where required.
  - First-party tracking only, without third-party trackers. If paid advertising is ever adopted, conversion measurement is a separate decision, implemented through server-side integrations and subject to consent.
- **Recommendations** start with simple techniques (for example "customers who bought this also bought" and category affinity). Example: a customer who bought a pillow may be interested in a blanket or a bed sheet set. Machine learning is a later phase.
- Metrics and behavior events are stored in **PostgreSQL, in a table partitioned by month**, written by a background job from the outbox rather than in the request; raw events are kept for 90 days and daily aggregates indefinitely; the console reads aggregates only. Export to an analytics warehouse that scales to zero is a later phase (see `docs/design.md`, section 6).

## 23. Parametrization

- The system is **configurable through the console wherever it makes sense**, so the administrator can change behavior without code changes.
- Parameters have defaults, validation and change history (audited).
- Not everything should become a parameter; the design must justify each one.
- Examples: media limits per product, limits for new stores, moderation rules, notification settings, the fee schedule, reputation metrics and thresholds, payout hold lengths, appeal deadlines, and the translation provider in the future.

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
- Webhooks are received on provider-specific endpoints and translated into internal events. The endpoint verifies the signature where the provider signs (Mercado Pago, Stripe, Adyen, PagBank), always re-reads the object from the provider's API before acting (some providers only send a shared token: Asaas, Iugu), stores the provider's event id for idempotency, and answers immediately while processing asynchronously, because acknowledgement deadlines are short (22 seconds at Mercado Pago, 10 seconds at Asaas). The same rules apply to shipping webhooks: signed at Melhor Envio and SuperFrete (HMAC-SHA256), shared token at Frenet, Basic auth at Loggi; 6 to 10 seconds to acknowledge; 5 retries at 15-minute intervals and then the event is discarded, so a missed event is recovered by polling.
- Tests use **fake implementations** of these interfaces, never real services. Provider sandboxes do not cover every flow (Iugu cannot create sub-accounts in sandbox, Pagar.me's PIX simulator does not work with split, Stripe cannot link platform and connected-account sandboxes), so end-to-end validation of split and payout follows a documented manual test plan against real accounts with small amounts (see [section 27](#27-development-workflow-and-quality)). Shipping sandboxes are partial too (Melhor Envio covers two carriers and no reverse, SuperFrete labels cannot be posted, Frenet has none), so the plan also covers label purchase, posting and delivery with real parcels.
- Official Go SDKs exist for some providers (Stripe, Adyen, Mercado Pago, Pagar.me in beta) and not for others (Asaas, Iugu, PagBank); an adapter written against the provider's OpenAPI definition is acceptable and is estimated as extra work. No shipping provider offers a Go SDK. Melhor Envio and SuperFrete require a `User-Agent` header naming the application and a contact e-mail.
- **E-mail:** Amazon SES and Resend fit the cost premise (no minimum or a free tier, a São Paulo region, an official Go SDK); bounce and complaint events become internal events that suppress further sends to that address (see [section 17](#17-notifications)); the sending domain of each marketplace needs DKIM, SPF and DMARC records in Cloudflare and production access at the provider, which is an infrastructure step of marketplace creation.
- Replacing the payment gateway is a migration project, not just a new adapter (see [section 11](#11-payments)).

## 26. Infrastructure and cost

- **Google Cloud Platform**, region **us-central1**.
- **Cloud Run**, scaling to zero.
- **Cloud SQL** (PostgreSQL) and **Cloud Storage**.
- **No Kubernetes and no load balancer.**
- **Infrastructure as code with Terraform.** Environment variables of the Cloud Run service are declared there with explicit values (see [section 7.1](#71-search-engine-indexing)).
- **Least privilege:** separate service accounts for Cloud Run, Cloud SQL and Cloud Storage, each with only the permissions it needs. Buckets are private by default; public access is granted only to what must be public (such as product images).
- **Backups and recovery:** automatic Cloud SQL backups with a defined retention period, and periodic restore tests. Media in Cloud Storage is covered by a defined retention and recovery approach.
- **Strong premise: the lowest possible operating cost**, avoiding recurring fees until the platform generates revenue. The design must identify fixed costs and keep them minimal. The cost inventory includes provider fees that scale with the number of stores rather than with sales (per sub-account monthly fees, per-payout fees, escrow fees, minimum invoices, dispute fees), which rules out gateways with per-sub-account monthly fees while the marketplace has many small or dormant stores. Shipping adds no fixed cost with the shortlisted providers; the inventory records the store-side wallet frictions (withdrawal fee at Melhor Envio, balance expiry at Loggi) and, later, a Correios contract minimum.

## 27. Development workflow and quality

- **CI** on every pull request: tests, lint, build, secret detection, **static security analysis** of the Go code and **vulnerability scanning** of dependencies (Go modules and container base image).
- Dependency updates are proposed automatically (for example by Dependabot) and go through the same CI.
- **Versioning:** the project follows **Semantic Versioning (SemVer)**. Versions are `0.y.z` during development; `1.0.0` marks the first production launch. Each version is a git tag `vMAJOR.MINOR.PATCH` on `main`.
- **Releases:** Claude Code creates a release, **only when the owner asks for one**, by pushing the version tag to `main`. The tag push triggers the CD pipeline, which builds the binary, creates the GitHub Release with generated notes, and deploys to production. Releases mark milestones, not individual changes; a long development cycle may have many merges and no release.
- **CD with two triggers:** every merge into `main` deploys automatically to the **lab** environment, without a tag or release; a release deploys to **production**.
- **Build identifier:** every build embeds an identifier derived from git (`git describe`): the clean version on a tagged commit (`v0.3.0`), or the version plus the distance and commit after it (`v0.3.0-12-gabc1234`); before the first tag, the short commit hash and build date. The identifier is recorded in the start-up log, shown in the footer of the administrative console and exposed on a version endpoint restricted to staff, so that the deployed build can always be verified, in the lab as well as in production.
- GitHub authenticates to GCP without stored keys (Workload Identity Federation), configured in Terraform and restricted to this repository.
- Environments: lab now; production in the future.
- **End-to-end tests** of real flows (sign-up, search, checkout) run in CI, written in **Python with Playwright** (the runtime already installed in the Claude Code environment), kept in `e2e/` and executed with `pytest` against the application started with fake providers.
- A **manual test plan** covers the provider flows that sandboxes cannot reproduce (split, KYC, payout hold and release; label purchase, posting and delivery), executed against real accounts with small amounts and real parcels before launch and after every provider change.
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

The topics closed during design, with their reasons, are recorded in `docs/design.md`, section 6. The topics below remain open because they depend on an input the design cannot produce; each names what closes it.

- Whether the Brazilian consumer right of withdrawal applies to vehicles, and its impact on the reservation model and on the payout hold (to be validated with a lawyer). Mercado Livre excludes vehicles from its buyer protection and eBay from its money-back guarantee, which supports a separate protection policy for vehicles. Until validated, the reservation is always refunded to the buyer (see [section 10](#10-cart-and-checkout)).
- Card installments: the plan ownership and the seller-funded opt-in are decided (see [section 11](#11-payments)); open are the provider cost tables and how the seller-funded cost is expressed (a monthly rate at PagBank, tiers at Asaas), which only the chosen gateway's commercial proposal answers.
- The maximum amount per transaction and per payment method the gateway allows, which is unpublished at most providers and bounds the vehicle reservation cap; answered by the commercial proposal.
- Payment gateway selection. `docs/research/payment-providers.md` shortlists Pagar.me and Asaas, with PagBank third if its single-seller-cart restriction can be lifted; Stripe and Adyen do not fit the Brazilian MVP as published but remain candidates for future markets. Selection criteria: sub-account per store created through the API, KYC run by the gateway with status events, payout hold released by the API, one payment split among several recipients, browser-side tokenization (PCI SAQ A), signed webhooks, no fixed cost before revenue, Brazilian card brands beyond Visa and Mastercard. Questions only a commercial proposal answers: marketplace plan costs, escrow and sub-account fees, chargeback fee, anticipation rate, maximum transaction amount, homologation timeline. The design works with any provider on the shortlist.
- Shipping provider selection (labels and tracking in the MVP are decided; see [section 12](#12-shipping)). `docs/research/shipping-providers.md` shortlists Melhor Envio first and SuperFrete second, with a direct Correios contract as a later step when volume justifies it; Kangu was discontinued in February 2025; Frenet and Intelipost carry subscription fees. To confirm in writing with Melhor Envio: the multi-store model for a marketplace and the partner programme terms.
- Auditing versus privacy: retention periods and legal retention obligations that delay key destruction (the mechanism is decided; see [section 21](#21-auditing)); to be set with a lawyer, with provisional defaults recorded in `docs/design.md`.
- Security incident response: the notification duties and deadlines under LGPD and, for future markets, other privacy laws (to be validated with a lawyer); the runbook structure and the incident record are designed.
- Re-evaluation of Cloud Run domain mapping (a preview feature) before production traffic and before adopting custom domains per marketplace (see [section 7](#7-tenancy-and-domains)): a spike before launch compares a global load balancer with Cloudflare as a proxy in front of the service.
- Phases and risks for future markets and cross-border sales, detailed when the first foreign market is chosen; the model keeps market, currency, language and tax as data (see [section 6](#6-internationalization)).
