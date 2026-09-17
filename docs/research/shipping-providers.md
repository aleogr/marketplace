# Shipping Providers

> **Status:** research document supporting `docs/requirements.md`, sections 12 (Shipping), 25 (External integrations) and the open topic "Shipping provider selection" in section 29. It records what each provider publishes; it does not decide. The closing sections give a shortlist and suggestions for the owner to accept or reject. An appendix covers transactional e-mail providers.
>
> **Access date for every source:** 2026-09-17. Each fact carries a reference such as `[ME-3]`; the [Sources](#sources) section gives the URL and what the page supports.
>
> **Legend:**
> - *Official* means a page published by the provider (pricing, developer documentation, terms, help center).
> - *(secondary)* marks facts from press, consultants or integrators, quoted because the official page was unreachable or silent.
> - `not found` means the data point was searched for and not located; nothing is estimated. "On request" means the official page itself says the value is negotiated.
> - Amounts are in BRL unless stated.

## Scope and method

What the platform needs from the shipping integration (from `docs/requirements.md`): quote cost and deadline per store shipment at checkout from origin and destination CEP, weight, dimensions and declared value; generate the label after payment, with the store shipping the parcel; tracking events delivered to the platform, because "delivered" releases the store payout; return labels for the consumer right of withdrawal; lowest fixed cost, paying per label; many third-party stores shipping from many origins; drop-off at agencies and pickup points; later, heavy and bulky items, store pickup and international shipping. Stores are companies (CNPJ) or individuals, and Brazilian shipments need an NF-e or a content declaration (DC-e).

Providers compared:

| Code | Provider | Type | Why it is here |
|---|---|---|---|
| ME | Melhor Envio | Aggregator | Most used aggregator by Brazilian small and mid sellers; multi-carrier; OAuth model for platforms |
| FR | Frenet | Quote gateway + labels | Quotes over the merchant's own carrier contracts; plan-based |
| SF | SuperFrete | Aggregator | Self-service, no monthly fee, OAuth model for multi-store platforms |
| KG | Kangu | Aggregator (Mercado Livre group) | Drop-off point network; API for integrators |
| CO | Correios (direct contract) | Carrier | The national carrier behind most aggregators; direct contract removes the intermediary |
| LG | Loggi | Carrier | Own network with API, drop-off points and same-day options in large cities |
| IP | Intelipost | TMS | Enterprise reference: quote engine and tracking over the merchant's own contracts |

Method: seven parallel research passes, one per provider, reading official pricing pages, developer documentation, terms and help centers directly; the first attempt was cut short by API limits and rerun. Where a provider only publishes "on request", that is recorded as the finding.

Dimensions: 1 pricing and fixed costs, 2 carriers and coverage, 3 quoting API, 4 labels and documents, 5 tracking and events, 6 reverse logistics, 7 marketplace and multi-seller model, 8 developer experience, 9 compliance and fiscal. Section 10 scores fit against the requirements and gives a shortlist; section 11 lists implications for `docs/requirements.md`; the appendix compares transactional e-mail providers.

---

## 1. Pricing and fixed costs

| | Cost model | Fixed costs and plans | Wallet and payment of labels | Insurance and extra fees |
|---|---|---|---|---|
| **ME** | Pay per label; no monthly fee ("Our revenue comes from the intermediation of each freight label"); the platform margin is inside the label price, not itemised (help-centre example: counter price R$ 4,00, cost R$ 1,00, customer pays R$ 2,00) [ME-21][ME-20][ME-22] | None; API free [ME-2]; wallet maintenance fee possible after 3 months without purchases (terms) [ME-20] | Prepaid "Melhor Carteira": PIX, credit card, boleto, up to R$ 50.000 per top-up; withdrawal R$ 6,90; API top-up by PIX or boleto [ME-23][ME-24][ME-16] | Declared value priced into the quote; Correios automatic cover R$ 25,63 (Mini Envios R$ 12,82); Jadlog risk fee above R$ 3.000 per kg; surcharges for non-mechanisable parcels; re-weighing credited or debited to the wallet [ME-26][ME-27][ME-30][ME-31][ME-37] |
| **FR** | Plan subscription plus labels bought on Frenet's contracts from a wallet; per-label intermediation fee exists in the terms but its amount is `not found` [FR-1][FR-3] | Iniciante R$ 0/month (labels, "até 90% de desconto"); Profissional R$ 85/month with 10.000 quotes (+R$ 19,80 per 1.000) and it is the tier that lists "cálculo automático de frete no checkout" and Correios label emission; Personalizado R$ 165 (25.000) and from R$ 440 (80.000 quotes); no cancellation fee; multi-origin rate tables R$ 205 activation + R$ 145 per update [FR-3][FR-15][FR-19] | Seller wallet via Mercado Pago, card, boleto, PayPal, lotérica (PIX not listed); API deposit through a Mercado Pago preference [FR-22][FR-6] | "SOS Proteção" optional, cost varies with declared value (rate `not found`); caps R$ 35.000 with NF-e, R$ 1.500 with a declaration [FR-17][FR-24] |
| **SF** | Pay per label; "sem mensalidades, gasto mínimo ou tarifas escondidas"; margin embedded in the label price (terms cl. 9) [SF-1][SF-28][SF-27] | None stated [SF-1] | Prepaid wallet: PIX R$ 5,00 to R$ 3.000,00, balance never expires; card in the panel; API checkout pays from balance only [SF-29][SF-8] | Correios declared value 2% (blog); surcharge R$ 22,60 above 70 cm; re-measurement debited and label generation suspended until paid [SF-38][SF-23][SF-27] |
| **KG** | Discontinued: Mercado Livre closed the intermediation platform on 23 Jan 2025, last orders 23 Feb 2025; every Kangu URL and the API endpoint now redirect to a Mercado Livre wind-down page (verified) [KG-1][KG-2][KG-6] | — | Historical: prepaid wallet, label released after manual payment (secondary) [KG-11] | `not found` |
| **CO** | Contract price table per service, invoiced monthly or paid at posting; no intermediary fee [CO-6][CO-8] | No setup or monthly fee documented, but a minimum monthly consumption ("contrapartida") set per contract, on request; published packages: Clube Correios and Platinum with no minimum and no price reducers, Diamante Start from R$ 100.000/month, Diamante from R$ 280.000/month; minimum waived for the first two billing cycles [CO-6][CO-2] | Monthly invoice with credit limit, or card hold at pre-posting [CO-18][CO-17b] | Ad valorem 2%; automatic cover R$ 25,63 (Mini Envios R$ 12,82); AR R$ 8,10 to 11,75; Mão Própria R$ 9,55 to 13,80; re-check tolerance 2 cm and 50 g [CO-14][CO-15][CO-2] |
| **LG** | Pay per label, "sem mínimo de pacotes", no contract for self-service; "a partir de R$ 5,89" (Loggi Ponto); tables for post-paid CNPJ on request [LG-9][LG-11][LG-10] | None for self-service; local PJ monthly billing R$ 9,90 [LG-12][LG-5] | PIX, wallet (balance expires in 90 days, non-refundable), credit card, boleto for business; post-paid monthly invoice after commercial approval [LG-5][LG-12] | Taxes and GRIS/ad valorem broken out in every quote; no separate insurance product; indemnity caps R$ 1.000 per item (prepaid) or R$ 3.000 (post-paid above R$ 30.000 revenue); undeliverable return charged 100% of the freight; pickup free from 10 parcels, otherwise R$ 49,90 [LG-15][LG-5][LG-6] |
| **IP** | TMS, not a carrier: quotes and labels run on the merchant's own carrier contracts (Correios PAC and SEDEX pre-configured at counter price); Intelipost sells no freight [IP-5][IP-6][IP-8] | Licence "franquia" (annual or monthly) plus a monthly variable fee by volume (quotes or orders) plus an implementation fee, all in the order form: on request; no public price page; annual IGP-M readjustment; SLA 99,5% [IP-14][IP-1] | Not applicable (SaaS billed by boleto or card; carriers billed under the merchant's contracts) [IP-14] | Correios declared value configured per method (minimum R$ 23,50); other fees carrier-table driven [IP-22] |

**Observations**

- Aggregators (Melhor Envio, SuperFrete) and Loggi charge nothing fixed: the platform's cost is the label price, with the aggregator's margin hidden inside the carrier discount. Frenet is the exception among aggregators: the features the platform needs (checkout quotes, Correios labels) sit in the R$ 85/month tier.
- A direct Correios contract removes the intermediary but adds a minimum monthly consumption negotiated per contract; the only packages without a minimum (Clube Correios, Platinum) carry no price reducers, so they are unlikely to beat aggregator prices.
- Wallet mechanics matter for many small stores: SuperFrete's balance never expires; Loggi's expires in 90 days; Melhor Envio charges R$ 6,90 to withdraw and may charge maintenance on idle balances.
- Re-measurement by the carrier is universal and is debited later from the wallet (Melhor Envio, SuperFrete, Loggi, Correios with a tolerance), which means the shipping cost of an order is not final at checkout.

## 2. Carriers and coverage

| | Carriers | Limits | Heavy, bulky, same-day, international |
|---|---|---|---|
| **ME** | Azul Cargo, Buslog, Correios (PAC, SEDEX, Mini Envios), Jadlog, J&T, LATAM Cargo, Loggi, Total Express; 14.000+ drop-off points [ME-28][ME-1] | Correios 30 kg, 100 cm per side, sum 200 cm; Mini Envios 300 g; Jadlog 100 kg at own units; J&T 30 kg; Loggi 30 kg; Buslog 50 kg; LATAM and Azul 60 kg. Cubic factors published per carrier (6000 for most; Jadlog .Package 3333 inter-state; Buslog 5000) [ME-30][ME-31][ME-32][ME-38][ME-39][ME-36][ME-29] | Up to 100 kg (Jadlog), 60 kg (LATAM, Azul); multi-volume only on Jadlog, Azul, Buslog, LATAM; same-day delivery `not found`; international not offered [ME-31][ME-12][ME-25] |
| **FR** | On Frenet's contracts: Correios, Jadlog (Package), Loggi, J&T, Total Express (PJ only), Optimus Cargas; on the merchant's own contracts: 40+ carriers tracked (Azul, Braspress, Buslog, Gollog, Jamef, Sequoia, TNT...) [FR-14][FR-20] | Correios 30 kg / 100 cm / 200 cm; Jadlog 80×80×80 cm, 30 kg at points, 120 kg at agencies; Loggi 30 kg; Total 30 kg, 120×80×60 cm; cubic weight L×W×H/6000 applied when it differs from real weight by 3 kg or more [FR-16][FR-25][FR-26][FR-27] | Jadlog agencies up to 120 kg; store pickup and motoboy articles exist (details `not found`); international service `not found` [FR-25][FR-13] |
| **SF** | Correios (PAC, SEDEX, Mini Envios), Jadlog, Loggi, J&T (API only) [SF-6][SF-21] | Correios 0,3 to 30 kg, sides 16–150 cm, sum ≤ 300 cm; Loggi 30 kg / 100 / 200 cm; Jadlog 120 kg at franchises; J&T 11 kg; cubic formula `not found` (computed by the API) [SF-7][SF-6] | Jadlog franchise up to 120 kg; furniture and appliances prohibited; same-day `not found`; international not offered [SF-6][SF-36][SF-19] |
| **KG** | Historical: Jadlog, Loggi, Rede Sul, Uello, Correios (secondary) [KG-13] | `not found` | `not found` |
| **CO** | Own network: SEDEX, PAC, SEDEX 10/12/Hoje, Mini Envios (contract only, from the Ouro category) [CO-3][CO-2] | 30 kg (PAC contract 50 kg within a state); max side 100 cm, sum 200 cm; Mini Envios 300 g, 24×16×4 cm; cubic factor `not found` (only a flag in the response) [CO-3][CO-23][CO-10] | SEDEX Hoje same day in selected localities; "Grande Formato" additional service (details `not found`); international via Exporta Fácil and Packet [CO-3][CO-2] |
| **LG** | Own network plus 400+ partner transporters and Correios redispatch; "100% cobertura nacional"; 1.700+ Loggi Pontos [LG-8][LG-2][LG-17] | 30 kg, 100 cm per side, sum 200 cm (API and terms); cubic divisor `not found` [LG-14][LG-15] | Same-day only local (Loggi Expresso); above 30 kg `not found`; international `not found` [LG-5] |
| **IP** | "+1.400 transportadoras" through freight tables loaded by Intelipost; named: Correios, Jadlog, Total Express, Loggi, J&T; Pegaki PUDO network for pickup points [IP-1][IP-12][IP-13] | Carrier-table dependent; limits and cubic divisor `not found`; volume types include pallet [IP-9] | Same-day and scheduled deliveries via real-time dispatch; heavy limits `not found`; international `not found` [IP-12] |

**Observations**

- National coverage is real everywhere through Correios or Loggi; the difference is in heavy parcels: only Melhor Envio (Jadlog 100 kg, LATAM and Azul 60 kg) and Frenet (Jadlog agencies 120 kg) go meaningfully above 30 kg, which matters for the "heavy and bulky" future need.
- Cubic weight rules are published only by Melhor Envio (per carrier) and Frenet (6000 with a 3 kg tolerance); Correios, Loggi and SuperFrete compute it server-side without publishing the factor. The platform should treat the quote as authoritative and not recompute it.
- Kangu is gone; the drop-off network it built is being absorbed by Mercado Envios and is not available to third-party stores.

## 3. Quoting API

| | Endpoint and inputs | Response | Limits and semantics |
|---|---|---|---|
| **ME** | `POST /api/v2/me/shipment/calculate` with origin and destination CEP and either `products[]` (Melhor Envio packs them) or `volumes[]`; `insurance_value` per product; optional receipt and own-hand [ME-11][ME-14] | Per service: price, `custom_price` (with the seller's markup), discount, `delivery_time` in days, `delivery_range`, packages, company [ME-14] | 250 requests per minute per user; one origin per call (per store shipment, not per multi-store cart); quote validity `not found`; business-day semantics `not found` [ME-6][ME-11] |
| **FR** | Public `POST /shipping/quote` (seller token) with CEPs, invoice value, items with weight and dimensions; whitelabel `POST /v1/quotes` (seller + partner tokens) with volumes and declared-value options [FR-9][FR-10] | Services with `ShippingPrice`, `ShippingCompetitorPrice` (counter price), `DeliveryTime` in days, `SessionId` for the whitelabel flow [FR-10] | Quotes count against the plan (10.000 to 80.000 per month); rate limits, validity and business-day semantics `not found`; per cart with per-product cubing summed [FR-3][FR-13][FR-29] |
| **SF** | `POST /api/v0/calculator` with CEPs, services, options (insurance, receipt, own hand) and either `package` or `products[]`; the API returns the ideal box that must be reused when creating the label [SF-6][SF-8] | Per service: price, discount, `delivery_time`, `delivery_range`, packages, company [SF-6] | Rate limits and validity `not found`; per package; new accounts limited to 5 simultaneous labels [SF-6][SF-33] |
| **KG** | Historical `POST /tms/transporte/simular` (secondary, from client code); now redirects [KG-14][KG-6] | — | — |
| **CO** | API Preço `GET /preco/v1/nacional/{product}` or batch POST (up to 100 items) with CEPs, weight, object type, dimensions, declared value, additional services; API Prazo separately returns `prazoEntrega` in business days [CO-10][CO-1] | Base and final price, billed weight, cubic flag, ad valorem, automatic insurance, additional services [CO-1][CO-10] | 150 requests per second per user and IP; contract token required; per object; validity `not found` [CO-10][CO-13] |
| **LG** | `POST /v1/companies/{id}/quotations` with addresses, `packages[]` (weight in g, dimensions in cm, `goodsValue`) and pickup types or external service ids [LG-15] | Per package: `totalAmount`, `baseAmount`, `sloInDays`, freight type, tax breakdown (PIS, COFINS, GRIS, ad valorem, ICMS, ISS) [LG-15] | Rate limit only as HTTP 429; validity `not found`; business-day semantics `not found`; documented maximum of 2 packages per request [LG-15][LG-3] |
| **IP** | `/quote` (by volume), `/quote_by_product`, `/pickup_quote_by_product` (recommended) with origin CEP per store or distribution centre, destination CEP, SKU, quantity, cost of goods, dimensions in cm and weight in kg [IP-7][IP-16] | `delivery_options[]` with `final_shipping_cost` and `provider_shipping_cost`, `delivery_estimate_business_days`, delivery method id; quote id must be stored for the order [IP-17][IP-7] | Rate-limit page exists but unreadable (`not found`); latency "under 100 ms", recommended 1,5 s timeout with back-off; offline fallback tables by CEP range [IP-20][IP-10] |

**Observations**

- All quoting APIs take the same inputs the requirements already mandate (CEPs, weight, dimensions, declared value), so the domain interface can be provider-neutral. Two providers pack items into boxes for the platform (Melhor Envio, SuperFrete), which removes the packing problem from the platform but requires the returned box to be reused at label time.
- Every quote is per shipment from one origin, which matches "shipping is quoted per store shipment". Frenet quotes a cart of items; the others quote packages.
- No provider publishes a quote validity. Prices can change between checkout and label purchase, and the carrier re-measures afterwards; the order should store the quoted service and price and tolerate a small difference at purchase.
- Correios is the only one to state that deadlines are business days; the others return "days" without saying which. The checkout promise must not assume calendar days.

## 4. Labels, documents and shipping flow

| | Flow and formats | Shipper on the label, NF-e and DC-e | Cancellation and expiry | Drop-off and pickup |
|---|---|---|---|---|
| **ME** | Cart → checkout (wallet) → generate (asynchronous) → print; PDF, ZPL and JPEG; DACE printable [ME-4][ME-15][ME-19][ME-42] | `from` block per cart item, but the terms forbid naming a sender other than the account holder; commercial shipments need `invoice.key` and state registration; non-commercial ones need full product data because Melhor Envio files the DC-e with SEFAZ (from 6 Apr 2026); ICMS-taxpayer companies cannot use DC-e [ME-15][ME-20][ME-43] | Cancellable while not posted, refund to the wallet within hours; expiry 20 days to pay, 20 to generate, 20 to post (7 for Correios) per the API FAQ (other official pages disagree) [ME-45][ME-46][ME-47][ME-6] | 14.000+ points (agencies, Pegaki partner points, Loggi Ponto, Jadlog shops); agencies API; pickup only through Loggi "Coleta" (same day if generated before 11h); Pegaki free pickup from 10 orders [ME-48][ME-49][ME-41][ME-50][ME-1] |
| **FR** | Whitelabel two-step (`/shipments` → `/shipments/checkout` → `/shipments/{id}/label`) or one-click with wallet debit; PDF A4 by default (ZPL `not found`); batch PDF; returns label, declaration and AR URLs and `ValidThrough` [FR-31][FR-34][FR-35] | Seller account or `From` per shipment; Correios declaration generated from the product list; Jadlog and Total require NF-e for companies; `IcmsExemption` flag [FR-31][FR-26b][FR-25][FR-27][FR-36] | Cancel before posting (API), refund as wallet credit; expiry value `not found` [FR-38][FR-39][FR-37] | Correios agencies in the origin municipality; branch lookup by state or CEP for Jadlog, Loggi, Total; seller pickup same day to 3 business days, cost `not found` [FR-26][FR-40][FR-26b] |
| **SF** | Cart → checkout (wallet) → print; PDF, A6 option for all labels; ZPL `not found` [SF-8][SF-9][SF-10] | `from` per shipment; the account holder's CPF is the sender for non-NF-e shipments (terms 11.18); `non_commercial = true` generates the DC-e automatically; NF-e passed as `invoice.number` and must be attached [SF-8][SF-27][SF-37] | Cancel before posting, refund to wallet; label valid 10 calendar days, then auto-cancelled and credited [SF-11][SF-16][SF-17] | Correios agencies, Pegaki points, Jadlog and Loggi points; pickup at the seller `not found` [SF-30][SF-41][SF-6] |
| **KG** | Historical: `POST /tms/transporte/solicitar`, prepaid, label after manual payment (secondary) [KG-14][KG-11] | Historical: `pedido.tipo` N (NF-e) or D (declaration) (secondary) [KG-14] | `not found` | Historical: pontos Kangu [KG-9] |
| **CO** | Pre-posting → asynchronous PDF label (standard or reduced) → post; ZPL `not found`; content declaration returned as HTML [CO-1] | Sender per pre-posting (may differ from the contract holder); NF-e or DC-e affixed outside the package (companies: NF-e; MEI: NF-e or DC-e); sender and recipient CPF/CNPJ mandatory [CO-17b][CO-2] | Pre-posting cancellable by id; card hold released; expiry 15 calendar days; delivery suspension irreversible and not refunded [CO-1][CO-17b][CO-2] | Agencies; "coleta" (scheduled, next day or same day before 11h) contract-only, price on request; lockers and pickup points free for the recipient [CO-3][CO-5][CO-14][CO-2] |
| **LG** | Authenticate → quote → `POST /async-shipments` (HTTP 202, wait for the webhook) → `POST /labels`; PDF A4 or A6 only, QR-code label for drop-off; ZPL unsupported [LG-4][LG-14][LG-16] | `shipFrom` per shipment; structured NF-e (44-character key, ICMS status) or content declaration per package [LG-14] | Cancel while not completed, "extra charges may apply"; refund rules and expiry `not found` [LG-19][LG-5] | 1.700+ Loggi Pontos with a locations API; scheduled pickup windows, free from 10 parcels, R$ 49,90 otherwise [LG-18][LG-8][LG-6] |
| **IP** | `POST /shipment_order` with quote id, origin (CEP, warehouse code, CNPJ), volumes and invoice; dispatch by PLP; `GET /shipment_order/get_label`; PDF or ZPL; label links expire after 30 days [IP-16][IP-9][IP-24][IP-25] | Shipper is the account's origin CNPJ; NF-e can be added after order creation; content declaration offered for non-DANFE operations (API page unreadable); Correios content rules since 26 Sep 2025 [IP-9][IP-12][IP-28] | Order cancellation status only (no label purchase to refund) [IP-27] | PLP pickup request to the carrier (NOTFIS, carrier API or Correios PPN); consumer drop-off and returns at Pegaki PUDOs; pickup cost `not found` [IP-29][IP-30][IP-31] |

**Observations**

- The fiscal rule is enforced by every provider's API: a company sends the NF-e key, an individual or a non-taxpayer sends product data and the provider files the electronic content declaration (DC-e), mandatory since 6 April 2026. The platform must know, per store, whether it is an ICMS taxpayer, and the store must be able to upload or reference an NF-e before the label is generated.
- Who appears as sender is a contractual point, not a technical one: Melhor Envio and SuperFrete bind the sender to the account holder, which is why their multi-seller model is one account per store (section 7). Correios and Loggi accept a sender per shipment under the contract holder's liability.
- Labels are PDF everywhere; ZPL exists only at Melhor Envio. Expiry ranges from 10 days (SuperFrete) to 15 (Correios) and 20 (Melhor Envio); cancellation before posting refunds the wallet.
- Pickup at the store's address is a contract feature (Correios) or priced per collection (Loggi R$ 49,90 under 10 parcels); aggregators route small sellers to drop-off points. The requirements' assumption that "the store ships" is consistent with the market.

## 5. Tracking and events

| | Tracking API | Webhooks | Event taxonomy and proof of delivery |
|---|---|---|---|
| **ME** | `POST /shipment/tracking` per order id (status and timestamps); order detail with billed weight and conciliation [ME-52][ME-53] | One URL per app; events order.created, pending, released, generated, received (drop point), posted, delivered, cancelled, undelivered, paused, suspended; HMAC-SHA256 `X-ME-Signature`; 6 s timeout; 5 attempts at 15-minute intervals; only labels bought through the app [ME-7] | No "out for delivery" or per-scan event (scans only on the public tracking page); no mandatory electronic proof of delivery; terms waive delay indemnities in Melhor Envio's favour [ME-7][ME-34][ME-20] |
| **FR** | `POST /tracking/trackinginfo` returns events with type [FR-41] | Tracking and status webhooks per shipment URL; only the last event is sent; 2xx within 10 s; optional shared header token, no signature; retries `not found` [FR-30][FR-42] | EventType 0 posted, 1 in transit, 2 delay, 3 returned, 4 lost, 5 out for delivery, 9 delivered, 18 at drop-off point; status webhook carries carrier-measured weight and price for reconciliation; AR when contracted [FR-42][FR-30][FR-31] |
| **SF** | `GET /api/v0/order/info/{id}` (status, tracking code, timestamps); carrier scan events `not found` (public page only) [SF-14][SF-32] | `POST /api/v0/webhook` returns a secret; events order.created, released, generated, posted, delivered, cancelled; HMAC-SHA256 `X-ME-Signature`; 30 s timeout, 5 retries every 15 minutes [SF-13][SF-12] | No failed-attempt, out-for-delivery or returned events; Correios AR optional; proof-of-delivery image `not found` [SF-12][SF-7] |
| **KG** | Historical polling endpoint; webhooks `not found` [KG-14] | — | — |
| **CO** | API Rastro polling (all events or last), restricted to the contract's own objects; polling limits `not found` [CO-1][CO-12] | "Webhook de rastreamento passivo" only for Diamante and Infinite categories (from R$ 280.000 per month); specification `not found` [CO-2] | Event codes (posted, in transit, out for delivery, delivered...); AR as paid add-on; up to 3 delivery attempts; contract service level with credits on the invoice [CO-1][CO-14][CO-2] |
| **LG** | `GET /packages/{code}/tracking` with status, history, promised date, receiver name and signature links; package detail with volumetric info and redispatch [LG-17][LG-23] | Registered by contacting sales; POST with packages and status objects; Basic auth chosen by the platform; signature and retries `not found` [LG-24] | 30 status codes (5 delivered, 8 returned, 12 delivery problem, 18 recipient absent, 22 awaiting sender action, 23 stolen, 29 held by tax authority); `actionRequired` reasons; receiver name and signature as proof [LG-25][LG-17] |
| **IP** | Carrier events enter the TMS (EDI, carrier APIs, Correios); `GET /shipment_order/read_status` for polling [IP-27][IP-32][IP-16] | Configured as "event rules" per macro or micro status, per order or volume, Basic or NTLM auth; 30-day delivery log with manual resend; signature and automatic retries `not found` [IP-33][IP-34][IP-35] | 13-step macro journey (new, invoiced, ready, created at carrier, in transit, collected, dispatched, out for delivery, delivered) plus micro statuses such as recipient absent and carrier delay; Correios AR as legal proof [IP-36][IP-27][IP-24] |

**Observations**

- A "delivered" event reaches the platform by webhook at Melhor Envio, SuperFrete, Frenet and Loggi; at Correios it requires polling unless the contract is Diamante or above. Since the payout hold releases on delivery, polling with a schedule is an acceptable fallback but must be designed.
- Only Loggi and Frenet expose intermediate events (out for delivery, failed attempt, returned) through the API; the two aggregators built for small sellers report lifecycle states, not scans. The buyer-facing tracking page can link to the provider's public page for details.
- Signed webhooks exist at Melhor Envio and SuperFrete (HMAC); Frenet and Loggi use a shared token or Basic auth. The webhook rules already written for payments (verify where possible, re-read the object, idempotency by event) apply unchanged.
- Proof of delivery with a signature is available only at Loggi through the API and at Correios as a paid AR; for disputes, the platform will mostly have the carrier's "delivered" status.

## 6. Reverse logistics and returns

| | Return label for the consumer | Who pays and flow | Via API |
|---|---|---|---|
| **ME** | Correios reverse (PAC or SEDEX only): no label; a posting code is shown in the panel and e-mailed by Correios; the consumer presents the code at a Correios counter with the printed DC-e; single volume; only after the original delivery [ME-8][ME-56] | The account that requests the reverse (the store) pays from its wallet; the consumer pays nothing; return goes with a DC-e even if the original had an NF-e; Azul and Buslog need a return NF-e [ME-8][ME-56][ME-36] | `POST /api/v2/me/cart/reverse`; not fully testable in sandbox [ME-8][ME-55] |
| **FR** | Correios PAC and SEDEX only, up to 30 kg; a posting code e-mailed by Correios; generated in the panel [FR-43] | Seller wallet pays, same flow as a label; cancel only before posting [FR-43] | `not found` (panel only) [FR-43] |
| **SF** | No native reverse product: the store generates a normal label with sender and recipient swapped and sends it to the customer [SF-26] | Store's wallet, same discounts; unused labels refunded on cancel or after 10-day expiry [SF-26][SF-16][SF-17] | Not offered as a distinct endpoint [SF-26] |
| **KG** | `not found` (product discontinued) | — | — |
| **CO** | "Logística Reversa" contract-only: authorised reverse at agency (e-ticket valid up to 30 days) or home collection; label generated and printed by Correios [CO-4][CO-16] | The authoriser (the store or the platform) pays; the consumer pays nothing; needs NF-e or DC-e [CO-4][CO-2] | SOAP web service (`solicitarPostagemReversa`, up to 50 requests per call); REST equivalent `not found` [CO-16] |
| **LG** | Consumer return label `not found` (no endpoint in the 15-endpoint reference; drop-off API lists a "Reversa" category); undeliverable parcels are returned to the sender at 100% of the freight [LG-1][LG-18][LG-5] | `returnTo` address per shipment; scheduled returns need 24 h notice [LG-14][LG-5] | `not found` |
| **IP** | "Reversa" module contracted separately: pickup at the consumer or drop-off at Correios agencies and PUDOs, e-ticket management; full features "apenas para ... Correios" [IP-31] | The shipper's own Correios reverse contract; Intelipost fee `not found` [IP-31] | `shipment_order_type` reverse; docs page unreadable (`not found`) [IP-9] |

**Observations**

- The consumer-facing return in Brazil is the Correios "reverse" code presented at a counter, without a printed label; it is exposed by API only at Melhor Envio (and by SOAP at Correios). Frenet does it in the panel, SuperFrete improvises with a swapped label, Loggi has no documented product.
- Returns travel with a DC-e even when the outbound went with an NF-e (Melhor Envio), and some carriers require a return NF-e. The returns flow of section 14 needs the store to produce the fiscal document, and the platform to request the reverse and pass the code to the buyer.
- Since the store pays the reverse from its wallet, "who pays return shipping by reason" (decided in section 14) is settled between store and platform, not with the provider.

## 7. Marketplace and multi-seller model

| | Documented model for platforms | One platform account for many stores | Onboarding of stores and billing |
|---|---|---|---|
| **ME** | One OAuth2 app per platform; each store authorises the app on its own Melhor Envio account (30-day tokens, 45-day refresh); marketplaces are an explicitly listed platform type with a quoting flowchart; "Parceria Verificada" pays a commission per shipment [ME-9][ME-3][ME-5][ME-2] | Staff answer on the forum: a single token "is possible" but discouraged (rate limit, one store's re-weighing debit blocks the whole account, excluded from the partner programme); terms forbid naming a sender other than the account holder; therefore on request [ME-37][ME-20] | Each store creates its own account (individual holder, CPF, 18+), verifies documents, funds its own wallet; new accounts limited to 3 simultaneous unposted labels, up to 50 after verification; multiple companies and addresses can be registered under one user [ME-20][ME-59][ME-58] |
| **FR** | Partner programme: the platform creates one Frenet account per store through `POST /partner/register` (partner token) and receives the store's token; whitelabel hides Frenet; label and wallet limits per store agreed with Frenet [FR-8][FR-44][FR-7][FR-11] | Central platform-paid wallet `not found`; quote engine needs paid multi-origin tables for dropshipping-style origins [FR-19] | Store fields: CPF/CNPJ, person type, address, plan code, confirmed e-mail and phone; each store's wallet pays; partner earns commissions; homologation in production by screen share [FR-8][FR-12][FR-5][FR-46] |
| **SF** | OAuth 2.0 "exclusive for operations with multiple stores"; each store authorises the platform (token does not expire); credentials requested by form; partner identification issued [SF-4][SF-5] | Prohibited: terms forbid reselling labels; the account holder's CPF is the sender for non-NF-e shipments [SF-27] | Store account with its own wallet; new accounts limited to 5 simultaneous labels until support raises it [SF-33] |
| **KG** | Historical: token per store; platform programme `not found` [KG-10][KG-11] | — | — |
| **CO** | The contract holder may authorise third parties ("autorizados") to post under its contract, bearing liability for rules, damages and complaints; explicit marketplace programme exists only for Correios Log+ (fulfillment); Diamante and Infinite packages exclude "resale of the service as main business" [CO-2] | Yes, as "autorizados" under the platform's contract, sender per pre-posting; sub-delegation keys valid up to 180 days [CO-2][CO-17b][CO-11] | One contract, one invoice; billing split `not found`; stores without CNPJ cannot hold their own contract but can ship as authorised users [CO-7][CO-2] |
| **LG** | "Integrador" model: one platform client id and secret; each store generates an integration code in its Loggi portal; the platform activates it and receives the store's company id; docs forbid asking stores for their credentials [LG-27][LG-28] | Platform-pays with arbitrary third-party origins `not found` [LG-27] | Each store needs a Loggi company account with a CNPJ (individuals cannot be integrated); billing on the store's own wallet or post-paid table [LG-27] |
| **IP** | Marketplace programme with partner marketplaces: each seller gets its own Intelipost account and hands its API key to the marketplace; account created within 48 h; free tables for Correios plus two carriers on some marketplaces [IP-8][IP-42] | Multi-origin is native inside one account (origin CEP, warehouse code and CNPJ per order); platform buying labels for sellers `not found` (Intelipost issues no labels of its own) [IP-9][IP-41] | SaaS billed to the contracting party; carriers billed under each shipper's contract; liability for carrier failures excluded [IP-14] |

**Observations**

- The market's answer to "many third-party stores" is not one platform account but one provider account per store, linked to the platform by OAuth (Melhor Envio, SuperFrete, Loggi) or created by the platform through a partner API (Frenet). Each store then funds its own wallet and is the legal shipper. This is the opposite of the payments model, where the platform creates sub-accounts and holds the money.
- The only documented "one account for everyone" is a direct Correios contract with authorised users, which puts liability and the minimum consumption on the platform, and requires a CNPJ.
- Individuals (CPF) can hold Melhor Envio, SuperFrete and Frenet accounts; Loggi's integrator model and a Correios contract require a CNPJ. Since the requirements allow individual sellers, the choice constrains who can sell.
- Consequence for the platform's onboarding: a store must complete a second onboarding at the shipping provider, and the platform must handle the state "shipping account not linked" or "wallet without balance" at label time, the same way it handles the payment sub-account states.

## 8. Developer experience

| | API and auth | Sandbox and homologation | SDKs, docs, status |
|---|---|---|---|
| **ME** | REST v2, OAuth2 authorization code with scopes, JWT 30 days; mandatory `User-Agent` with contact e-mail; 250 requests per minute [ME-5][ME-3][ME-6] | Sandbox with fictitious balance, Correios and Jadlog only, statuses advance every 15 minutes; reverse generation unavailable [ME-61][ME-55] | PHP SDKs only (Go `not found`); docs in Portuguese with OpenAPI 3.1 per page and an LLM index; status page; support without consulting [ME-63][ME-62][ME-65][ME-6] |
| **FR** | REST, header tokens (seller and partner), four hosts (quote, whitelabel, configuration, registration); OpenAPI 3.0 per page [FR-9][FR-10][FR-36][FR-8][FR-35] | No sandbox ("não possui ambiente de homologação"); test accounts on request; mandatory homologation in production by screen share [FR-47][FR-48][FR-46] | PHP SDK and platform plugins; no Go; rate limits and status page `not found` [FR-50] |
| **SF** | REST v0, Bearer JWT plus mandatory `User-Agent`; OAuth token never expires; gRPC-style error codes [SF-3][SF-5] | Sandbox with simulated PIX top-up; labels not postable [SF-3] | No SDKs, no status page; OpenAPI per page; Portuguese docs; 20+ plugins [SF-6][SF-34] |
| **KG** | Historical REST with a `token` header; docs gone [KG-14] | — | — |
| **CO** | REST (SOAP for reverse); Basic credentials exchanged for a 24-hour JWT bound to contract and posting card; 2FA mandatory on the portal since 27 Apr 2026 [CO-13][CO-12][CO-9] | Homologation environment with test credentials [CO-1][CO-16] | No SDKs; Swagger visible only after login; Portuguese only; availability dashboard inside the portal [CO-1][CO-12][CO-9] |
| **LG** | REST, OAuth2 client credentials (v2 JWT); credentials only through commercial contact; 429 without published numbers [LG-4][LG-28][LG-3] | Staging environment [LG-29] | No SDKs, OpenAPI download and status page `not found`; Portuguese docs with an LLM index [LG-1][LG-3] |
| **IP** | REST v1 with an `Api-Key` header and platform headers; official auth and rate-limit pages unreadable [IP-18][IP-16] | Sandbox `not found`; homologation checklists published [IP-43][IP-7] | Java, .NET, Ruby SDKs (no Go); Portuguese docs on Stoplight; status page with contractual 99,5% SLA [IP-16][IP-42][IP-4][IP-14] |

**Observations**

- No shipping provider offers a Go SDK; every adapter will be hand-written against embedded OpenAPI definitions, as already accepted for some payment providers.
- Sandboxes are weaker than in payments: Frenet has none and homologates in production; Melhor Envio's sandbox covers two carriers and no reverse; Loggi's credentials come only through sales. The manual test plan added to section 27 must include shipping.
- Melhor Envio and SuperFrete require a `User-Agent` identifying the application and contact; both docs and Loggi's warn against collecting stores' credentials, which the OAuth model avoids.

## 9. Compliance and fiscal

| | NF-e and DC-e | Prohibited items and insurance | Data protection |
|---|---|---|---|
| **ME** | NF-e (models 55 and NFA-e) or DC-e; DC-e mandatory since 6 Apr 2026 and filed with SEFAZ by Melhor Envio; ICMS-taxpayer companies cannot use DC-e; MEI may [ME-66][ME-43][ME-67] | Per-carrier prohibited list (updated 10 Sep 2026); indemnity paid by the carrier per its policy and credited to the wallet; DC-e shipments on Jadlog have no damage cover [ME-68][ME-69][ME-31] | Terms reference the LWSA group privacy policy; individual account holder with CPF, 18+ [ME-20][ME-70] |
| **FR** | Correios accepts either (declaration from the product list); Jadlog and Total require NF-e for companies; `DeclarationUrl` returned [FR-26b][FR-25][FR-27][FR-31] | Correios and Jadlog lists; SOS Proteção caps R$ 35.000 (NF-e) or R$ 1.500 (declaration); claims within 30 days [FR-26b][FR-25][FR-17] | Parties as independent controllers; international cloud transfers under contract [FR-1] |
| **SF** | DC-e mandatory since 6 Apr 2026, issued automatically; DC-e not allowed where NF-e is required (CNPJ obliged to issue it) [SF-24][SF-25][SF-37] | Prohibited list incl. furniture and appliances; Correios cover R$ 25,63, Jadlog up to R$ 1.500 (loss only), Loggi covers damage with a declaration [SF-36][SF-20][SF-31][SF-40] | Minimum data processing clause; named DPO [SF-27][SF-39] |
| **KG** | Historical: NF-e or declaration by type (secondary) [KG-14] | `not found` | Data-deletion form on the wind-down page [KG-1] |
| **CO** | NF-e or DC-e affixed outside the package; companies NF-e, MEI either, exempt companies DC-e; CPF/CNPJ of both parties mandatory; posting refused without a document [CO-2][CO-9] | Prohibited and restricted lists; ad valorem 2%; no cover for poor packaging or hidden damage [CO-19][CO-14][CO-2] | Contract clause on controller and operator roles, incident communication, deletion at contract end [CO-2] |
| **LG** | NF-e or content declaration per package as structured fields; DC-e from 6 Apr 2026 (terms updated 20 Jul 2026) [LG-14][LG-5][LG-32] | Prohibited list incl. jewellery; RCTR-C and RC-V cargo insurance; caps R$ 1.000 or R$ 3.000 per item; claim within 30 days [LG-5][LG-21] | Loggi as processor, shipper as controller; privacy notice [LG-5] |
| **IP** | NF-e per volume, updatable after creation; content declaration for non-DANFE operations; Correios content-description and hazardous-goods rules since 26 Sep 2025 [IP-9][IP-12][IP-28] | Correios prohibited list; declared value per method [IP-28][IP-22] | Customer as controller, Intelipost as operator; ISO 27001 cited [IP-14][IP-45] |

**Observations**

- The fiscal document is the hard constraint of Brazilian shipping: since April 2026 every parcel without an NF-e needs an electronic content declaration, and the aggregators file it for the store. The platform's order needs a fiscal field per shipment (NF-e key, or product data for the DC-e) and the store profile needs the ICMS-taxpayer flag.
- Indemnity is the carrier's, capped by document type: shipments under a content declaration have lower caps (R$ 1.000 to 1.500) and sometimes no damage cover. For electronics with high value, the store must ship with an NF-e and declared value, which argues for CNPJ stores in the electronics marketplace.

---

## 10. Fit against the requirements and shortlist

**Yes** means documented on an official page; **Partial** means possible with a workaround or a commercial condition; **No** means documented as unavailable; `not found` means the official pages are silent. Kangu is listed for completeness only.

| Requirement (requirements.md section) | ME | FR | SF | KG | CO | LG | IP |
|---|---|---|---|---|---|---|---|
| Quote per store shipment from CEPs, weight, dimensions, declared value (§12) | Yes | Yes | Yes | Discontinued | Yes (Preço + Prazo) | Yes | Yes |
| Label after payment through the API (§12) | Yes: PDF, ZPL, JPEG | Yes: PDF | Yes: PDF | Discontinued | Yes: PDF | Yes: PDF | Yes: PDF, ZPL, on own contracts |
| Tracking events by webhook, including delivered (§11, §12) | Yes, signed | Yes, unsigned, last event only | Yes, signed | No | Partial: polling; webhook only from Diamante | Yes, Basic auth, via sales | Yes, event rules, unsigned |
| Return label for the consumer through the API (§14) | Yes: Correios reverse code | Partial: panel only | Partial: swapped label | No | Partial: SOAP, contract | `not found` | Partial: separate module |
| No fixed cost, pay per label (§26) | Yes | No: checkout quotes in the R$ 85 tier | Yes | — | Partial: minimum consumption on request | Yes | No: licence plus volume fee |
| Many stores from many origins (§12) | Partial: one account per store via OAuth; single account discouraged and restricted by terms | Partial: one account per store via partner API, whitelabel | Partial: one account per store via OAuth; resale prohibited | — | Yes: authorised users under the platform's contract, platform liable | Partial: one account per store, CNPJ only | Partial: per-seller accounts with API key |
| Individuals (CPF) as stores (§8, §20) | Yes | Yes | Yes | — | No (contract needs CNPJ) except as authorised users | No | `not found` |
| Drop-off points and pickup (§12) | Yes: 14.000+ points; pickup via Loggi only | Yes: branches; pickup cost `not found` | Yes: points; pickup `not found` | — | Agencies; pickup contract-only | Yes: 1.700+ points; pickup R$ 49,90 under 10 parcels | PUDOs; pickup by PLP |
| NF-e and DC-e handled by the API (§9 compliance) | Yes, DC-e filed by the provider | Yes, declaration generated | Yes, DC-e filed by the provider | — | Yes, DC-e HTML | Yes, structured | Partial: page unreadable |
| Heavy and bulky above 30 kg (§12 future) | Yes: up to 100 kg | Yes: up to 120 kg at agencies | Partial: Jadlog franchise 120 kg | — | 50 kg PAC within a state | `not found` | Carrier-dependent |
| Sandbox (§27) | Yes, partial coverage | No | Yes | — | Yes | Staging | `not found` |
| Official Go SDK (§24) | No | No | No | — | No | No | No |

**Reading of the matrix**

- Two aggregators cover the MVP without fixed cost and with the whole flow through the API: **Melhor Envio** (eight carriers, ZPL, signed webhooks, reverse code through the API, 14.000+ points, DC-e filed for the store) and **SuperFrete** (four carriers, signed webhooks, simplest API, no reverse product). Melhor Envio is the more complete of the two; SuperFrete is a credible fallback and a second adapter for the interface.
- Both impose the same architecture: one provider account per store, linked by OAuth, with the store's own wallet. The platform does not buy labels on behalf of stores; it quotes with the store's token and drives the store's account. A single platform account is contractually restricted at both.
- **Loggi** is a strong carrier for large cities but limits integration to CNPJ stores, gives credentials only through sales and documents no consumer return label; it fits better as a carrier inside an aggregator than as the MVP integration.
- A direct **Correios** contract is the only "platform is the shipper" model: authorised users post under the platform's contract, with the platform liable and a minimum consumption to negotiate; tracking is polling unless the volume reaches the Diamante tier. It is the natural second step when volume justifies a contract, not the first integration.
- **Frenet** and **Intelipost** are quote engines over the merchant's own contracts with subscription pricing; they solve a problem the platform does not have yet (many carrier contracts to manage) and add fixed cost. Intelipost is the enterprise reference for later phases.
- **Kangu** was shut down by Mercado Livre in February 2025 and is not a candidate.

**Suggested shortlist and next step**

1. Melhor Envio as the first integration, SuperFrete as the second adapter behind the same interface (both OAuth per store, both with signed webhooks).
2. Confirm with Melhor Envio, in writing, the multi-store model for a marketplace (OAuth per store versus a platform account with registered companies and addresses) and the partner programme terms.
3. Design the store onboarding with a "shipping account" step (authorise the provider, fund the wallet, verify documents), and the checkout with the quote made using the store's token.

## 11. Implications for the requirements

Each item names the section of `docs/requirements.md` it affects and whether the analysis suggests **changing** a decision, **confirming** a proposal or open topic, or **adding** something the document does not cover yet. Nothing here is decided; the owner decides.

### 11.1 Shipping (§12) and the provider open topic (§29)

1. **Change (§12):** the market's model for many third-party stores is one shipping account per store, linked to the platform by OAuth or created through a partner API, with the store's own wallet paying for labels. The platform is not the shipper. Suggestion: state that the platform integrates the store's shipping account (authorisation, quotes with the store's token, label purchase from the store's wallet) rather than buying labels itself, and keep "platform as shipper under its own carrier contract" as a later option tied to a Correios contract.
2. **Add (§12, §20):** store onboarding gains a shipping step: authorise the provider, verify documents (Melhor Envio limits new accounts to 3 simultaneous labels until verified, SuperFrete to 5), fund the wallet. Listings must not go live for physical products until the store can generate a label, or the checkout must handle "store cannot ship yet".
3. **Confirm (§12, labels and tracking in the MVP):** every shortlisted provider exposes quote, label and tracking through the API, with a signed "delivered" webhook at Melhor Envio and SuperFrete. Add: polling as a fallback for providers without webhooks (Correios) and a scheduled reconciliation of stale shipments.
4. **Add (§12):** the shipping cost of an order is not final at checkout: carriers re-measure and the difference is debited from the store's wallet later. The order stores the quoted service, price and deadline; a post-delivery adjustment line exists for the difference, with a rule for who absorbs it (the store, by default).
5. **Add (§12):** deadlines returned by providers are "days" without stating business days (only Correios says business days). The checkout promise is expressed as a date range computed by the platform from the provider's `delivery_range`, never as calendar arithmetic on a single number.
6. **Add (§12, §8.2):** quotes are per package and providers pack items into a box (Melhor Envio, SuperFrete) or accept pre-packed volumes; multi-volume shipments are limited to some carriers. The shipment model supports several volumes and records the box returned by the quote, which must be reused at label time.
7. **Add (§12):** a label has an expiry (10 days at SuperFrete, 15 at Correios, 20 at Melhor Envio) and can be cancelled before posting with a refund to the wallet. The order workflow needs a "label expired" state and a re-purchase path, and the store's dispatch deadline (already a reputation metric) must be shorter than the label expiry.
8. **Confirm and detail (§12 future needs):** pickup at the store's address is a paid or contract feature (Loggi R$ 49,90 under 10 parcels; Correios contract); drop-off at agencies and partner points is free and universal. The MVP assumes drop-off; pickup stays a later phase.
9. **Add (§29):** the shipping provider selection open topic can be narrowed: `docs/research/shipping-providers.md` shortlists Melhor Envio first and SuperFrete second, with a direct Correios contract as a later step when volume justifies it; Kangu is discontinued; Frenet and Intelipost carry subscription fees.

### 11.2 Orders, returns and disputes (§14)

10. **Add (§14):** the consumer return in Brazil is a Correios "reverse" posting code presented at a counter, not a printed label; it is requested by the store through the API (Melhor Envio) and paid from the store's wallet; it travels with a content declaration even when the outbound had an invoice. The returns flow: store (or platform on its behalf) requests the reverse, the buyer receives the code and the declaration, the tracking of the reverse feeds the refund.
11. **Add (§14, §11):** proof of delivery with a signature exists only at Loggi through the API and at Correios as a paid add-on; the usual evidence is the carrier's "delivered" status plus the tracking history. The evidence pack decided in §14 should store the full tracking history received by webhook, not only the final status.
12. **Add (§14):** carrier indemnity is capped by document type (about R$ 25 automatic at Correios; R$ 1.000 to 1.500 under a content declaration; higher with an NF-e and declared value) and paid to the store's wallet by the carrier, not by the platform. The dispute rules must say that a lost parcel is settled between store and carrier, and what the buyer gets meanwhile.

### 11.3 Catalog and fiscal (§8, §9 compliance, §18)

13. **Add (§8.2, §14):** since 6 April 2026 every parcel without an NF-e needs an electronic content declaration (DC-e), which the aggregators file with SEFAZ from the product data; ICMS-taxpayer companies cannot use it. The store profile needs an "ICMS taxpayer" flag and the shipment needs either the NF-e key or the product data for the declaration; the store must be able to attach the NF-e before generating the label.
14. **Add (§20, electronics):** under a content declaration some carriers give no damage cover and low caps; high-value electronics should ship with an NF-e and declared value, which in practice means CNPJ stores for the electronics marketplace, or at least a warning to individual sellers.
15. **Add (§8.2):** carriers publish cubic-weight factors (6000 for most; Jadlog 3333 inter-state at Melhor Envio) and dimensional minimums (Correios 11×6×0,4 cm; Loggi 10 cm width, 15 cm length). The listing validation for weight and dimensions should enforce provider minimums and the product model should keep the volumetric weight as computed by the provider.

### 11.4 Integrations and cost (§25, §26, §27)

16. **Add (§25):** the shipping webhook rules mirror payments: signed at Melhor Envio and SuperFrete (HMAC-SHA256), shared token at Frenet, Basic auth at Loggi; acknowledgement within 6 to 10 seconds; 5 retries at 15-minute intervals then discard, so a missed event must be recovered by polling.
17. **Add (§25, §24):** no shipping provider offers a Go SDK; adapters are written against the OpenAPI definitions embedded in their docs. Melhor Envio and SuperFrete require a `User-Agent` naming the application and a contact e-mail.
18. **Add (§27):** shipping sandboxes are partial: Melhor Envio covers two carriers and no reverse; SuperFrete labels cannot be posted; Frenet has none. The manual test plan covers label purchase, posting and delivery with small parcels before launch.
19. **Add (§26):** shipping adds no fixed cost with the shortlisted providers; the costs to inventory are the store-side wallet frictions (withdrawal R$ 6,90 at Melhor Envio, balance expiry at Loggi) and, later, a Correios contract minimum.
20. **Add (§17):** the shipping provider sends its own notifications to buyers in some cases (Melhor Envio public tracking page, Loggi status updates); the platform's transactional e-mails should link to the provider's tracking page rather than duplicate scan events.

### 11.5 Gaps in this analysis

- Melhor Envio's position on a single platform account comes from a staff answer on its community forum, not from a contract; it must be confirmed in writing.
- Intelipost's API reference and Correios' Swagger are behind client-side rendering or login; endpoint details for both come from guides, SDK READMEs and manuals.
- Loggi's and Frenet's help centers blocked automated reading; affected facts are marked as search snippets.
- No provider publishes a real price example, so cost comparison needs sandbox or real quotes on the platform's expected routes.

---

## Appendix: transactional e-mail providers

Scope: transactional e-mail only (order paid, shipped, delivered, sign-up, password reset, second-factor codes) in pt-BR and en-US from a Go backend on Cloud Run; marketing only with consent; lowest fixed cost; bounce and complaint webhooks; DKIM, SPF and DMARC on the platform's own domains, one per marketplace later. Prices are in USD as shown on the official pages on 2026-09-17; nothing is estimated.

| Provider | Free tier | Entry paid tier and overage | Dedicated IP | Bounce and complaint webhook | Signature | Official Go SDK | Regions | Notes |
|---|---|---|---|---|---|---|---|---|
| Amazon SES | No permanent free tier; up to USD 200 in credits for 6 months on new accounts [EM-1] | USD 0,10 per 1.000 à la carte, no minimum; Essentials USD 0,16, Pro USD 0,22 (minimum USD 105/month) [EM-1] | USD 24,95 per IP per month [EM-1] | Through SNS or event publishing (Firehose, CloudWatch), no direct HTTPS webhook; events include Bounce, Complaint, Delivery, DeliveryDelay [EM-7] | SNS message signing (not covered by the fetched pages: `not found`) | Yes, aws-sdk-go-v2 [EM-8] | US, EU, Brazil (sa-east-1) [EM-6] | Sandbox per region: 200 messages per day to verified recipients until production access is requested (answer within 24 h); every sending identity verified per region [EM-2] |
| Postmark | 100 e-mails per month [EM-3] | USD 15 per month for 10.000; overage USD 1,80 per 1.000 [EM-3] | From USD 50 per IP, requires 300.000 per month [EM-3] | Yes: Delivery, Bounce, Spam Complaint, Open, Click, Subscription Change [EM-4][EM-9] | None ("does not support HMAC"); basic auth in the URL plus IP allowlist; retries 1 to 15 minutes [EM-9] | No official (community only) [EM-10][EM-11] | `not found` [EM-3] | Transactional and broadcast streams never mix; 45-day retention [EM-3][EM-4] |
| Resend | 3.000 per month, 100 per day, 3 domains, 30-day retention [EM-5] | USD 20 per month for 50.000, 10 domains; overage USD 0,90 to 0,46 per 1.000 [EM-5] | USD 30 per month, Scale plan only [EM-5] | Yes; event names `not found` on the fetched page [EM-13] | Svix signatures (`svix-id`, `svix-timestamp`, `svix-signature`) [EM-13] | Yes, resend-go v3 [EM-14] | US, EU (Ireland), Brazil (São Paulo), Tokyo; region per domain [EM-12] | Domain caps per plan (3, 10, 1.000); inbound included [EM-5] |
| Brevo | 300 per day after manual account approval [EM-15] | Prices rendered client-side and absent from the fetched page: `not found` [EM-15] | Enterprise feature, price `not found` [EM-15] | Yes: Sent, Delivered, Hard and Soft Bounce, Complaint, Blocked, Error and more [EM-16] | None documented; IP allowlist recommended [EM-16] | Yes, brevo-go (Swagger-generated) [EM-17] | `not found` [EM-15] | Account approval gate before sending [EM-15] |
| SendGrid (Twilio) | 100 per day for 60 days (trial) [EM-19] | Essentials USD 19,95 per month for 50.000; overage USD 1,30 per 1.000 [EM-19] | One included in Pro (USD 89,95 per month); extra IP price `not found` [EM-19] | Event Webhook; event list `not found` on the fetched page [EM-20] | ECDSA signature header and timestamp, or OAuth 2 client credentials [EM-20] | Yes, sendgrid-go [EM-21] | US; EU data residency on Pro and Premier; Brazil `not found` [EM-19] | Log retention 3 days on Essentials [EM-19] |
| Mailgun | 100 per day, 1 domain, 1-day logs [EM-22] | Basic USD 15 per month for 10.000 (1 domain); Foundation USD 35 for 50.000 with 1.000 domains; overage from USD 1,30 per 1.000 [EM-22] | One included from Foundation; extra USD 59 per IP per month [EM-22] | Yes; event list `not found` (page 404) [EM-23] | HMAC signing key; verification helper in the Go SDK [EM-23] | Yes, mailgun-go v5 [EM-23] | US and EU from one account; Brazil `not found` [EM-22] | Message retention only on Scale [EM-22] |

**Observations**

- For a platform that scales to zero, Amazon SES (USD 0,10 per 1.000, no minimum, region in São Paulo, official Go SDK) and Resend (3.000 free per month, Go SDK, signed webhooks, São Paulo region, domain caps) are the two that fit the cost premise; Postmark and Mailgun start at USD 15 per month, SendGrid at USD 19,95.
- Multiple sending domains, one per marketplace later, are unlimited at SES (identities per region) and capped at Resend (3 free, 10 on Pro, 1.000 on Scale); Mailgun allows 1 domain on the cheap tiers.
- Signed webhooks exist at Resend, SendGrid and Mailgun; Postmark and Brevo rely on IP allowlists; SES delivers events through SNS rather than a plain HTTPS webhook, which is extra plumbing on Cloud Run.
- Every provider gates new accounts (SES sandbox, Brevo approval, Postmark and Resend daily caps), so the sending domain and production access should be set up well before launch, as a manual step.
- Suggested reading for `docs/requirements.md` §25: keep e-mail behind the interface as already required; SES or Resend as the first provider; record bounce and complaint events as internal events that suppress further sends to that address (§17); the sending domain per marketplace needs DKIM, SPF and DMARC records in Cloudflare, which is an infrastructure step of marketplace creation (§29).

---

## Sources

All sources were accessed on 2026-09-17. Entries marked *(secondary)* or *(search snippet only)* are not official pages of the provider. Where a page shows its own update date, it is noted.

### Melhor Envio (ME)

- **[ME-1]** https://melhorenvio.com.br/ — homepage claims, Pegaki free pickup min 10 orders, Melhor Rastreio
- **[ME-2]** https://docs.melhorenvio.com.br/docs/introducao-a-api.md — free API, partner programme
- **[ME-3]** https://docs.melhorenvio.com.br/docs/autenticacao-1.md — OAuth2, token validity
- **[ME-4]** https://docs.melhorenvio.com.br/docs/conceitos-gerais-do-melhor-envio.md — cart/checkout/generate concepts
- **[ME-5]** https://docs.melhorenvio.com.br/reference/fluxo-de-autorização.md — authorize URL, scopes (updated 2026-02-20)
- **[ME-6]** https://docs.melhorenvio.com.br/reference/faq.md and /reference/introducao-api-melhor-envio.md — rate limit 250/min, expiry 20 days, base URLs, User-Agent (FAQ updated 2026-07-14)
- **[ME-7]** https://docs.melhorenvio.com.br/docs/webhooks.md — events, signature, retries (updated 2026-06-02)
- **[ME-8]** https://docs.melhorenvio.com.br/docs/logistica-reversa-carrinho.md — reverse code flow
- **[ME-9]** https://docs.melhorenvio.com.br/docs/tipos-de-plataformas.md and /docs/integracao.md — one app per platform, state param
- **[ME-11]** https://docs.melhorenvio.com.br/docs/cotacao-de-fretes.md — quote modes, one freight per request
- **[ME-12]** https://docs.melhorenvio.com.br/docs/compra-de-fretes.md — 7-day cart validity, NF rules, multi-volume limits, Azul restriction
- **[ME-14]** https://docs.melhorenvio.com.br/reference/calculo-de-fretes-por-produtos.md — calculate request/response schema, marketplace flowchart
- **[ME-15]** https://docs.melhorenvio.com.br/reference/inserir-fretes-no-carrinho.md — cart payload rules, DC-e from 06/04/2026 (updated 2026-07-14)
- **[ME-16]** https://docs.melhorenvio.com.br/reference/inserir-saldo-na-carteira-do-usuario.md — balance top-up API
- **[ME-19]** https://docs.melhorenvio.com.br/reference/impressao-de-etiquetas-em-arquivo.md — PDF/ZPL/JPEG
- **[ME-20]** https://static.melhorenvio.com.br/termos-de-uso.pdf — terms of use dated 13 Feb 2026: entity, sender rule, remuneration, indemnity, limits, privacy
- **[ME-21]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220727703700 — no monthly fee (updated 2025-10-03)
- **[ME-22]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220761778580 — how discounts work
- **[ME-23]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220445829652 and /articles/31220446895636 — payment methods and release times
- **[ME-24]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220463176980 and https://lp.melhorenvio.com.br/taxas-e-detalhes/ — wallet limits, R$6.90 withdrawal, maintenance exempt
- **[ME-25]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220771994004 — no international
- **[ME-26]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220744216084 — Jadlog risk fee (updated 2026-01-12)
- **[ME-27]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220417305492 — insured value, Correios automatic insurance (updated 2026-06-24)
- **[ME-28]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220760653588 — 8 partner carriers
- **[ME-29]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220744875540 — billed weight and cubic factors (updated 2026-08-11)
- **[ME-30]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220449136660 — Correios rules
- **[ME-31]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220462480788 — Jadlog rules
- **[ME-32]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220400389524 — J&T rules
- **[ME-34]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220431326356 — Mini Envios (updated 2026-05-19)
- **[ME-36]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220457649044 — Azul Cargo rules (updated 2026-07-22)
- **[ME-37]** https://docs.melhorenvio.com.br/discuss/67d3401c47113c0012428dc5 — official forum answer on single-token marketplace model (secondary: community post answered by ME staff)
- **[ME-38]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220418026644 — Buslog rules (updated 2026-06-24)
- **[ME-39]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220454442644 — LATAM Cargo rules (updated 2026-05-27)
- **[ME-41]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220727731092 — pickup availability, Loggi Coleta timing
- **[ME-42]** https://docs.melhorenvio.com.br/reference/impressao-dace.md — DACE printing
- **[ME-43]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/41306548209300 — DC-e rules (updated 2026-04-07)
- **[ME-45]** https://docs.melhorenvio.com.br/reference/cancelamento-de-etiquetas.md and /reference/verificar-se-etiqueta-pode-ser-cancelada.md — cancel API, 12 h refund (updated 2026-06-18)
- **[ME-46]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220449413396 — cancellation/refund rules (updated 2026-02-09)
- **[ME-47]** https://docs.melhorenvio.com.br/reference/geracao-de-etiquetas.md — 20-day validity after generation, cancel restrictions
- **[ME-48]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220712014868 — drop-off locations and limits (updated 2026-02-23)
- **[ME-49]** https://docs.melhorenvio.com.br/reference/listar-agencias-e-opcoes-de-filtro.md — agencies API
- **[ME-50]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220745885332 and /articles/31220745834004 — posting/pickup per carrier
- **[ME-52]** https://docs.melhorenvio.com.br/reference/rastreio-de-envios.md — tracking endpoint response
- **[ME-53]** https://docs.melhorenvio.com.br/reference/listar-informacoes-de-uma-etiqueta.md — order fields
- **[ME-55]** https://docs.melhorenvio.com.br/reference/inserir-logistica-reversa-no-carrinho.md — reverse API payloads (updated 2026-08-21)
- **[ME-56]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220449351700 — reverse logistics in the panel (updated 2026-04-23)
- **[ME-58]** https://docs.melhorenvio.com.br/reference/cadastrar-loja.md and /reference/cadastrar-endereco-de-uma-loja.md — companies/addresses API
- **[ME-59]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220442517652 — simultaneous shipment limit (3 at signup)
- **[ME-61]** https://docs.melhorenvio.com.br/docs/sandbox.md — sandbox behaviour (updated 2026-06-05)
- **[ME-62]** https://docs.melhorenvio.com.br/llms.txt — docs index
- **[ME-63]** https://docs.melhorenvio.com.br/docs/sdk.md — PHP SDKs only
- **[ME-65]** https://status.melhorenvio.com.br/ → https://melhorenvio.instatus.com — status page
- **[ME-66]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220384848148 — accepted fiscal documents (updated 2026-07-02)
- **[ME-67]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220462592276 — Correios content declaration, MEI
- **[ME-68]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220391789332 — prohibited products guide (updated 2026-09-10)
- **[ME-69]** https://centraldeajuda.melhorenvio.com.br/hc/pt-br/articles/31220445877268 — theft/loss indemnity (updated 2026-03-27)
- **[ME-70]** https://docs.melhorenvio.com.br/docs/cadastro-no-ambiente-de-produção-do-melhor-envio.md — production signup with CPF and documents

### Frenet (FR)

- **[FR-1]** https://www.frenet.com.br/termos-de-uso/ - terms: legal entity, not a carrier, billing, cancellation, liability, LGPD; "última atualização 26/08/2026"
- **[FR-3]** https://www.frenet.com.br/planos - plan prices and quote quotas
- **[FR-5]** https://docs.frenet.com.br/docs/getting-started.md - free partner API, commissions
- **[FR-6]** https://docs.frenet.com.br/docs/gestão-de-carteira.md - wallet fields, Mercado Pago deposit flow
- **[FR-7]** https://docs.frenet.com.br/docs/cadastro-de-contas-na-frenet.md - composite key, label limits 10/20
- **[FR-8]** https://docs.frenet.com.br/reference/cadastrar-um-novo-cliente.md - onboarding API fields
- **[FR-9]** https://docs.frenet.com.br/reference/calculateshippingquote.md - POST /shipping/quote spec
- **[FR-10]** https://docs.frenet.com.br/reference/getshipmentquoteasync.md - whitelabel POST /quotes spec
- **[FR-11]** https://docs.frenet.com.br/docs/onboarding.md - required onboarding fields
- **[FR-12]** https://docs.frenet.com.br/reference/createdepositasync.md - POST /wallet/deposit
- **[FR-13]** https://ajuda.frenet.com.br/categorias/como-usar-a-frenet/ and https://ajuda.frenet.com.br/categorias/frete/ - article index; subtotal rule
- **[FR-14]** https://ajuda.frenet.com.br/categorias/transportadoras-contrato-frenet/ - six Frenet-contract carriers
- **[FR-15]** https://ajuda.frenet.com.br/knowledge-base/a-frenet-possui-fidelidade-ou-taxa-de-cancelamento/ - no loyalty/cancellation fee
- **[FR-16]** https://ajuda.frenet.com.br/knowledge-base/qual-o-limite-de-peso-e-dimensoes-dos-correios/ - Correios limits, cubic rule /6000, 3 kg threshold
- **[FR-17]** https://frenet.com.br/termos-de-uso-sos-protecao/ - SOS Proteção coverage/limits
- **[FR-19]** https://ajuda.frenet.com.br/knowledge-base/a-frenet-trabalha-com-dropshipping/ - multi-origin tables R$ 205/145
- **[FR-20]** https://ajuda.frenet.com.br/knowledge-base/2090928-transportadoras-com-rastreio/ - tracked carriers list
- **[FR-22]** https://ajuda.frenet.com.br/knowledge-base/quais-sao-as-formas-de-pagamento-das-etiquetas/ - payment methods
- **[FR-24]** https://ajuda.frenet.com.br/knowledge-base/qual-o-custo-do-sos-protecao/ - SOS cost varies by declared value (search snippet only; page returned 404 on fetch)
- **[FR-25]** https://ajuda.frenet.com.br/knowledge-base/regras-de-embarque-jadlog/ - Jadlog rules
- **[FR-26]** https://www.frenet.com.br/blog/regras-de-postagem-etiquetas-de-frete-frenet/ - posting rules per carrier incl. Loggi limits
- **[FR-26b]** https://ajuda.frenet.com.br/knowledge-base/regras-de-postagem-correios/ - Correios posting, documents, cancel refund 1 business day
- **[FR-27]** https://ajuda.frenet.com.br/knowledge-base/regras-de-postagem-total-express/ - Total Express rules
- **[FR-29]** https://ajuda.frenet.com.br/knowledge-base/2580298-como-e-calculado-o-peso-de-meu-produto-carrinho/ - per-product cubing (search snippet only)
- **[FR-30]** https://docs.frenet.com.br/docs/webhook-atualização-de-dados-de-pedidos.md - status webhook
- **[FR-31]** https://docs.frenet.com.br/reference/createshipmentasync.md - POST /shipments, statuses, label URLs
- **[FR-34]** https://docs.frenet.com.br/docs/orders-oneclick-geração-automática-de-etiquetas.md and https://docs.frenet.com.br/reference/createorderoneclickasync.md - one-click
- **[FR-35]** https://docs.frenet.com.br/llms.txt - endpoint index (batch label PDF, branches by CEP, cancel batch)
- **[FR-36]** https://docs.frenet.com.br/reference/getservices.md and .../updateadditionalservices.md - services, IcmsExemption, AR, mão própria
- **[FR-37]** https://ajuda.frenet.com.br/knowledge-base/quais-sao-os-status-das-etiquetas-frenet/ - panel statuses, refund to wallet
- **[FR-38]** https://docs.frenet.com.br/reference/cancelshipmentasync.md and .../removeshipmentasync.md - cancel/delete
- **[FR-39]** https://ajuda.frenet.com.br/knowledge-base/como-cancelar-minha-etiqueta/ - cancel only before posting
- **[FR-40]** https://docs.frenet.com.br/reference/getbranches.md - branches
- **[FR-41]** https://docs.frenet.com.br/reference/trackorder.md - tracking API
- **[FR-42]** https://docs.frenet.com.br/docs/webhook-atualização-de-tracking.md - tracking webhook, event types
- **[FR-43]** https://ajuda.frenet.com.br/knowledge-base/como-funciona-a-logistica-reversa/ - reverse logistics
- **[FR-44]** https://docs.frenet.com.br/reference/getuserexists.md - user exists
- **[FR-46]** https://docs.frenet.com.br/docs/processo-de-homologação.md - homologation in production
- **[FR-47]** https://ajuda.frenet.com.br/knowledge-base/a-frenet-possui-uma-ambiente-de-homologacao/ - no sandbox
- **[FR-48]** https://docs.frenet.com.br/docs/pré-requisitos-checklist-para-integração.md - cadastro-hml registration
- **[FR-50]** https://github.com/orgs/FrenetGatewaydeFretes/repositories - PHP SDK and plugins, no Go

### SuperFrete (SF)

- **[SF-1]** https://superfrete.com/ — free, no monthly fee, up to 80%, carriers, 3M downloads, payment methods, company CNPJ
- **[SF-3]** https://superfrete.readme.io/reference/primeiros-passos.md — base URLs, sandbox, headers, error codes, free API
- **[SF-4]** https://superfrete.readme.io/reference/autenticação.md — OAuth only for multi-store platforms; manual token for single sellers; credential form
- **[SF-5]** https://superfrete.readme.io/reference/solicitação-do-token.md — OAuth authorize/token URLs, token never expires
- **[SF-6]** https://superfrete.readme.io/reference/cotacao-de-frete.md — calculator fields, service ids, carrier limits, example response, OpenAPI
- **[SF-7]** https://superfrete.readme.io/reference/informações-dos-pacotes.md — Correios weight/dimension/declared-value limits, AR/MP/VD
- **[SF-8]** https://superfrete.readme.io/reference/adicionar-frete-carrinho.md — cart fields, documents, NF-e vs DC-e, statuses
- **[SF-9]** https://superfrete.readme.io/reference/apiintegrationv1checkout.md — checkout from wallet, response with tracking
- **[SF-10]** https://superfrete.readme.io/reference/tag-link.md — print endpoint, PDF, A6 Zebra option
- **[SF-11]** https://superfrete.readme.io/reference/cancelar-pedido.md — cancel rules, wallet refund
- **[SF-12]** https://superfrete.readme.io/reference/webhook.md — events, payload, X-ME-Signature HMAC, retries
- **[SF-13]** https://superfrete.readme.io/reference/criar-webhook-app.md — create webhook, secret_token
- **[SF-14]** https://superfrete.readme.io/reference/tag-informações-do-pedido.md — order info fields, statuses
- **[SF-16]** https://ajuda.superfrete.com/artigo/posso-cancelar-a-minha-etiqueta-2/ — cancellation, wallet credit
- **[SF-17]** https://ajuda.superfrete.com/artigo/a-etiqueta-tem-validade-2/ — 10 calendar days validity
- **[SF-19]** https://ajuda.superfrete.com/artigo/o-super-frete-emite-fretes-internacionais-2/ — no international
- **[SF-20]** https://ajuda.superfrete.com/artigo/extravio-encomenda/ — loss process, indemnity amounts
- **[SF-21]** https://ajuda.superfrete.com/artigo/quais-as-transportadoras-disponiveis-na-superfrete/ — Correios, Jadlog, Loggi
- **[SF-23]** https://ajuda.superfrete.com/artigo/em-que-situacao-pode-ser-aplicada-a-taxa-adicional-pelos-correios/ — R$ 22.60 surcharge
- **[SF-24]** https://ajuda.superfrete.com/artigo/dc-e-e-obrigatoria/ — DC-e mandatory from 2026-04-06
- **[SF-25]** https://ajuda.superfrete.com/artigo/quem-nao-pode-usar-declaracao-de-conteudo-eletronica/ — NF-e-obliged cannot use DC-e
- **[SF-26]** https://ajuda.superfrete.com/artigo/logistica-reversa-na-superfrete-como-funciona-o-frete-reverso-e-etiqueta-reversa/ — no native reverse; swap addresses
- **[SF-27]** https://superfrete.com/termos-de-uso-e-condicoes — terms: integrator role, cl. 6 reconciliation, 9 remuneration, 10 indemnity/wallet, 11.18 CPF as sender, 12.3–12.4 no resale/transfer, 12.6 API terms, 11.10 LGPD. No update date shown
- **[SF-28]** https://ajuda.superfrete.com/artigo/preciso-pagar-para-usar-o-super-frete/ — no monthly fee
- **[SF-29]** https://ajuda.superfrete.com/artigo/como-pago-a-etiqueta-usando-pix/ — Pix recharge R$ 5–3,000, never expires
- **[SF-30]** https://ajuda.superfrete.com/artigo/desconto-ponto-de-postagem/ — larger discount at drop-off points
- **[SF-31]** https://ajuda.superfrete.com/artigo/como-funciona-indenizacao-jadlog/ — Jadlog indemnity
- **[SF-32]** https://ajuda.superfrete.com/artigo/como-rastrear-uma-mercadoria/ — tracking tab, rastreamento.superfrete.com
- **[SF-33]** https://ajuda.superfrete.com/artigo/duvidas-comuns-ao-integrar-a-superfrete-com-plataformas-de-e-commerce/ — seller pays, 5-label initial limit, plugin carriers
- **[SF-34]** https://ajuda.superfrete.com/artigo/quais-as-integracoes-disponiveis-da-superfrete-com-plataformas-de-e-commerce/ — 20+ integrations, API
- **[SF-36]** https://ajuda.superfrete.com/artigo/politica-itens-permitidos-restritos-proibidos/ — prohibited items
- **[SF-37]** https://ajuda.superfrete.com/artigo/passo-a-passo-para-emitir-dc-e-com-a-superfrete/ — automatic DC-e/DACE
- **[SF-38]** https://superfrete.com/blog/seguro-correios — blog dated 2026-07-24; declared value 2% (secondary: blog, not help center)
- **[SF-39]** https://superfrete.com/politica-de-privacidade — LGPD, DPO
- **[SF-40]** https://ajuda.superfrete.com/artigo/como-funciona-a-loggi-na-superfrete/ — Loggi docs, limits, attempts, indemnity, active CPF/CNPJ
- **[SF-41]** https://ajuda.superfrete.com/artigo/consulta-pontos-de-coleta-disponiveis/ — SuperFrete collection points in São Paulo

### Kangu (KG)

- **[KG-1]** https://www.mercadolivre.com.br/ajuda/38279 — "Ajuda Kangu" wind-down page (forms: become ML pickup point, driver, payment history, "Tenho valores para receber da Kangu", data deletion, settle debts). Target of all Kangu redirects
- **[KG-2]** https://www.ecommercebrasil.com.br/noticias/mercado-livre-encerra-operacoes-da-kangu — 2025-01-23 — (secondary) reproduces the full official Mercado Livre statement: shutdown from 23.01.2025, last orders 23.02.2025, 80% staff reallocated, 30-90 days notice, carrier role and Places continue
- **[KG-6]** https://portal.kangu.com.br/tms/transporte/simular, https://portal.kangu.com.br/docs/api/transporte/, https://ajuda.kangu.com.br/, https://www.kangu.com.br/faq/, https://www.kangu.com.br/cadastro-seller/ — all 301 to the ML help page (verified with curl; POST to simular also 301)
- **[KG-9]** https://www.nuvemshop.com.br/blog/kangu/ — 2025-01-23 — (secondary) how Kangu worked, 2500+ points / 600+ stations / 11 states, no new sign-ups after Jan 2025, credit refund within 10 days
- **[KG-10]** https://ajuda.bling.com.br/hc/pt-br/articles/4422594551191--DESCONTINUADA-Integra%C3%A7%C3%A3o-com-a-Kangu — (secondary) discontinuation notice, token location, label models (padrão, A4, A6, PIMACO), features (quote per volume/product, tracking sync, label print)
- **[KG-11]** https://ajuda.climba.com.br/helpcenter/como-configurar-a-integracao-com-a-kangu/ — updated 2024-10-03 — (secondary) token creation ("API – Credenciais / Tokens", "Nova Credencial"), carrier preferences, prepaid balance and manual label payment
- **[KG-13]** https://www.agenciaeplus.com.br/vnda-kangu-integracao-com-solucao-de-frete-promete-envios-ate-75-mais-baratos/ — 2024-05-17 — (secondary) carriers list (Jadlog, Loggi, Rede Sul, Uello, Correios Mini/PAC/Sedex), "até 75% mais baratos", founded 2019
- **[KG-14]** https://github.com/ecomplus/app-kangu (files: functions/routes/ecom/modules/calculate-shipping.js, functions/lib/kangu/create-tag.js, functions/lib/kangu/import-order-status.js, functions/ecom.config.js, hosting/description.md; raw.githubusercontent.com master) — (secondary) endpoints simular/solicitar/rastrear, header `token`, request/response fields, `tipo` N/D, status values, docs URL `portal.kangu.com.br/docs/api/transporte/#/`

### Correios (CO)

- **[CO-1]** https://www.correios.com.br/atendimento/developers/arquivos/manual-para-integracao-correios-api (Manual de Integração v2.4, Revisão 04/2025; local dump correios-integracao.txt) - APIs, auth, endpoints, prepostagem, rastro
- **[CO-2]** https://www.correios.com.br/enviar/precisa-de-ajuda/contrate-os-correios/arquivos/contratos-formalizados-a-partir-de-marco-de-2020/termo-de-condicoes-comerciais (Termo de Condições Comerciais 09/09/2026, Apêndices 13/08/2026; local dump correios-termo.txt) - packages, minimums, webhook, reversa, third parties, LGPD
- **[CO-3]** https://www.correios.com.br/enviar/encomendas/nacional - service limits (weights, dimensions), Mini Envios contract-only
- **[CO-4]** https://www.correios.com.br/enviar/encomendas/logistica-reversa - reversa modalities, who pays, 30-day validity
- **[CO-5]** https://www.correios.com.br/enviar/encomendas/coleta/coleta - coleta modalities, cut-offs
- **[CO-6]** https://www.correios.com.br/enviar/precisa-de-ajuda/perguntas-frequentes-como-contratar-os-correios - FAQ: minimum value on request, billing cycle, credit
- **[CO-7]** https://www.correios.com.br/enviar/precisa-de-ajuda/contrate-os-correios - eligibility (CNPJ), phones, steps
- **[CO-8]** https://www.correios.com.br/enviar/precisa-de-ajuda/saiba-mais-como-contratar-os-correios - contracting steps, benefits
- **[CO-9]** https://www.correios.com.br/atendimento/developers - API list public/private, 2FA, NF-e validation 15/09/2025
- **[CO-10]** https://www.correios.com.br/atendimento/developers/manuais/manual-api-preco-1 (v1.0, 24/11/2025) - Preço endpoints, params, codes, 150 req/s
- **[CO-11]** https://www.correios.com.br/atendimento/developers/manuais/correioswebservice (April 2026) - auth, 24 h token, subdelegation 180 days
- **[CO-12]** https://www.correios.com.br/atendimento/developers/manuais/manual-correios-web-service-cws (Abril 2026) - API list, Rastro restriction
- **[CO-13]** https://www.correios.com.br/atendimento/developers/manuais/manual-uso-da-api-token (v1.0, 03/11/2025) - token endpoints, Basic auth, 429
- **[CO-14]** https://www.correios.com.br/enviar/servicos-adicionais (updated 12/04/2026) - VD max values, 2% ad valorem, AR/MP prices
- **[CO-15]** https://www.correios.com.br/receber/encomenda/indenizacoes (vigência 12/04/2026) - automatic indemnity, delay percentages
- **[CO-16]** https://www.correios.com.br/atendimento/developers/arquivos/manual-de-implementacao-do-web-service-logistica-reversa.pdf (Revisão 23/04/2025; local text correios-reversa-ws.txt) - SOAP reversa WS
- **[CO-17b]** https://www.correios.com.br/ppn and 17b https://www.correios.com.br/atendimento/developers/manual-do-usuario-ppn - PPN, label formats, cancellation, 15-day expiry, sender data
- **[CO-18]** https://www.correios.com.br/enviar/precisa-de-ajuda/arquivos/passo-a-passo-contratacao-das-solucoes-dos-correios.pdf (local text correios-passo-a-passo.txt) - SEI contracting flow, credit
- **[CO-19]** https://www.correios.com.br/enviar/proibicoes-e-restricoes - prohibited/restricted items
- **[CO-23]** https://blog.correios.com.br/2021/08/31/correios-mini-envios-pequenos-produtos-grandes-vendas/ (search snippet only; page 404) - Mini Envios 300 g, 15x10x1 to 24x16x4 cm

### Loggi (LG)

- **[LG-1]** https://docs.api.loggi.com/llms.txt - index of all API pages (15 endpoints)
- **[LG-2]** https://www.loggi.com/ - scale claims, products, promo
- **[LG-3]** https://docs.api.loggi.com/reference/orientações.md - API overview, HTTP codes, 429
- **[LG-4]** https://docs.api.loggi.com/reference/authenticatev2.md - OAuth2 v2, base URLs
- **[LG-5]** https://www.loggi.com/termos-condicoes-clientes/ - Terms (updated 2026-07-20): parties, caps, payment, cancellation, DC-e, LGPD
- **[LG-6]** https://www.loggi.com/loggifacil - pickup fee, payment methods
- **[LG-8]** https://www.loggi.com/produtos-loggi/ - products, R$5.89, pickup windows, postpaid
- **[LG-9]** https://www.loggi.com/loggiponto - from R$5.89, 1,700 points, no minimum
- **[LG-10]** https://ajuda.loggi.com/hc/pt-br/articles/15890393258637-Entenda-a-tabela-de-pre%C3%A7os-e-prazos - standard vs personalised tables (search snippet only; page blocked)
- **[LG-11]** https://www.loggi.com/transportadora - no minimum, 75% claim
- **[LG-12]** https://www.loggi.com/para-todos/ - payment methods incl. boleto, coupon, "logística reversa descomplicada"
- **[LG-14]** https://docs.api.loggi.com/reference/createasyncshipment.md - shipment fields, limits, document types
- **[LG-15]** https://docs.api.loggi.com/reference/quote.md - quotation API
- **[LG-16]** https://docs.api.loggi.com/reference/criaretiqueta-1.md - labels API (PDF, A4/A6)
- **[LG-17]** https://docs.api.loggi.com/reference/trackpackagetrackingcode.md - tracking API, POD fields
- **[LG-18]** https://docs.api.loggi.com/reference/listdropofflocations.md - Loggi Pontos API
- **[LG-19]** https://docs.api.loggi.com/reference/packagecancel.md - cancellation
- **[LG-21]** https://ajuda.lojaintegrada.com.br/pt-BR/articles/12580777-ressarcimento-por-extravio-e-avaria-na-loggi - indemnity process (secondary)
- **[LG-23]** https://docs.api.loggi.com/reference/packagedetailstrackingcode.md - package details
- **[LG-24]** https://docs.api.loggi.com/reference/webhook.md - webhook setup and payload
- **[LG-25]** https://docs.api.loggi.com/reference/trackingapi.md - status codes 1-30, actionRequired
- **[LG-27]** https://ajuda.loggi.com/hc/pt-br/articles/33869890508045-C%C3%B3digo-de-Integra%C3%A7%C3%A3o - integration code, PJ account required (via r.jina.ai)
- **[LG-28]** https://ajuda.loggi.com/hc/pt-br/articles/24666891957389-Confira-nosso-Guia-de-Integra%C3%A7%C3%B5es - credentials only via commercial; postpaid API evaluation (via r.jina.ai)
- **[LG-29]** https://docs.api.loggi.com/reference/authenticatev1.md - v1 token 300 s, deprecated
- **[LG-32]** https://ajuda.lojaintegrada.com.br/pt-BR/articles/12580612-documentacao-necessaria-para-envios-pela-loggi - NF-e for CNPJ, declaration for CPF (secondary, 2025-11-03)

### Intelipost (IP)

- **[IP-1]** https://www.intelipost.com.br/ — home: products, "+1.400 transportadoras", claims; /planos and /precos return 404
- **[IP-4]** https://status.intelipost.com.br/ (-> https://uptime.intelipost.com.br/) — status components
- **[IP-5]** https://ajuda.intelipost.com.br/pt-BR/articles/5527501-sobre-a-intelipost — "não é uma transportadora", modules
- **[IP-6]** https://ajuda.intelipost.com.br/pt-BR/articles/5527330-painel-da-intelipost-tms-tudo-sobre-o-modulo-cotacao-de-frete — freight tables, quote types, business days
- **[IP-7]** https://docs.intelipost.com.br/docs/tms-embarcador/ZG9jOjMwNzcxNTM4-homologacao-cotacao — quote endpoints, inputs, response fields
- **[IP-8]** https://ajuda.intelipost.com.br/pt-BR/articles/5527482-como-configurar-sua-conta-intelipost-para-receber-cotacoes-de-frete-em-uma-integracao-via-marketplace — marketplace seller accounts, API key to marketplace, free carrier tables, Correios counter contract
- **[IP-9]** https://docs.intelipost.com.br/docs/tms-embarcador/ZG9jOjMwNzcxNTM5-homologacao-pedido-e-despacho — order fields, multi-CD, NF, PLP, label endpoint, reverse type
- **[IP-10]** https://docs.intelipost.com.br/docs/tms-embarcador/ZG9jOjMwNzcxNTQy-contingencia-fallback — fallback tables and formula
- **[IP-12]** https://www.intelipost.com.br/produtos/despacho/ — carriers named, labels, declaração de conteúdo, same-day
- **[IP-13]** https://www.intelipost.com.br/solucoes/solucoes-para-marketplace/ — marketplace solution, PUDO network
- **[IP-14]** https://www.intelipost.com.br/wp-content/uploads/2024/04/INTELIPOST-20240415-MASTER-SERVICE-AGREEMENT.pdf (index: https://www.intelipost.com.br/master-service-agreement/) — fees model, implementation, payment, SLA 99.5%, liability, LGPD roles
- **[IP-16]** https://raw.githubusercontent.com/intelipost/sdk-java/master/README.md — official SDK: endpoints, API V1 URL
- **[IP-17]** https://raw.githubusercontent.com/intelipost/sdk-ruby/master/README.md — official SDK: quote response sample fields
- **[IP-18]** https://raw.githubusercontent.com/intelipost/sdk-dotnet/master/README.md — official SDK: Api-Key and Platform/Plugin headers, order fields
- **[IP-20]** https://ajuda.intelipost.com.br/pt-BR/articles/5687732-melhores-praticas-para-consumo-de-nossa-api — latency <100 ms, 1.5 s timeout, backoff
- **[IP-22]** https://ajuda.intelipost.com.br/pt-BR/articles/5527477-como-configurar-a-taxa-de-seguro-dos-correios — insurance min R$23,50
- **[IP-24]** https://ajuda.intelipost.com.br/pt-BR/articles/5527393-como-gerar-as-etiquetas-dos-pedidos — PDF/ZPL, AR, 500 per batch
- **[IP-25]** https://ajuda.intelipost.com.br/pt-BR/articles/14602789-como-funciona-a-validade-dos-links-de-etiquetas-no-tms — 30-day label link validity
- **[IP-27]** https://ajuda.intelipost.com.br/pt-BR/articles/5596810-relacao-de-micro-status-disponiveis-para-cadastro-no-painel-da-intelipost-tms — macro/micro status IDs
- **[IP-28]** https://ajuda.intelipost.com.br/pt-BR/articles/12261797-novos-campos-obrigatorios-na-integracao-com-os-correios — Correios rules 26/09/2025, hazardous flag, prohibited items
- **[IP-29]** https://ajuda.intelipost.com.br/pt-BR/articles/5527377-painel-da-intelipost-tms-tudo-sobre-o-modulo-operacao — PLP types, dispatch
- **[IP-30]** https://ajuda.intelipost.com.br/pt-BR/collections/3110229-gestao-de-pedidos-e-operacao — PPN/PLP article list
- **[IP-31]** https://ajuda.intelipost.com.br/pt-BR/articles/5527419-o-que-e-logistica-reversa-e-como-obter-esse-modulo-na-minha-conta-do-painel-da-intelipost-tms — reverse module scope, Correios-only note
- **[IP-32]** https://ajuda.intelipost.com.br/pt-BR/collections/3110230-entregas-e-reversa — tracking/webhook article list
- **[IP-33]** https://ajuda.intelipost.com.br/pt-BR/articles/5527430-como-configurar-o-envio-de-notificacoes-via-webhook — webhook event rules
- **[IP-34]** https://ajuda.intelipost.com.br/pt-BR/articles/6065334-como-enviar-webhook-atraves-do-painel-da-intelipost-tms — manual webhook send, auth formats
- **[IP-35]** https://ajuda.intelipost.com.br/pt-BR/articles/12241246-como-realizar-a-gestao-de-disparos-e-configuracoes-de-seus-webhooks — webhook logs 30 days, resend, headers
- **[IP-36]** https://ajuda.intelipost.com.br/pt-BR/articles/13062132-guia-de-macros-status-do-tms-visao-completa-da-jornada-do-pedido — macro status journeys
- **[IP-41]** https://ajuda.intelipost.com.br/pt-BR/articles/10658422-orientacoes-sobre-a-tabela-de-frete-intelipost-multi-origens — multi-origin tables
- **[IP-42]** https://docs.intelipost.com.br/docs/tms-embarcador/ZG9jOjMwNzcxNTM3-integracoes-para-marketplaces and https://docs.intelipost.com.br/docs/tms-embarcador (nav only; API reference pages incl. ZG9jOjMwNzcxNTMy-api-endpoint-e-autenticacao, 76zptlhp4j62u-rate-limit, ciydury03om06-declaracao-de-conteudo-eletronica, 7cbf437232c61-criar-pedido-de-reversa, bab2afb7f67fe-criar-pedido-de-entrega did not render)
- **[IP-43]** https://ajuda.intelipost.com.br/pt-BR/articles/5527503-documentacao-tecnica-para-integracao-via-api — docs allow testing services
- **[IP-45]** https://www.intelipost.com.br/politica-de-privacidade/ — LGPD, ISO 27001

### E-mail providers (appendix) (EM)

- **[EM-1]** https://aws.amazon.com/ses/pricing/
- **[EM-2]** https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html
- **[EM-3]** https://postmarkapp.com/pricing
- **[EM-4]** https://postmarkapp.com/developer
- **[EM-5]** https://resend.com/pricing
- **[EM-6]** https://docs.aws.amazon.com/general/latest/gr/ses.html
- **[EM-7]** https://docs.aws.amazon.com/ses/latest/dg/monitor-sending-activity.html
- **[EM-8]** https://github.com/aws/aws-sdk-go-v2
- **[EM-9]** https://postmarkapp.com/developer/webhooks/webhooks-overview
- **[EM-10]** https://postmarkapp.com/developer/integration/official-libraries
- **[EM-11]** https://github.com/mattevans/postmark-go
- **[EM-12]** https://resend.com/docs/dashboard/domains/regions
- **[EM-13]** https://resend.com/docs/dashboard/webhooks/verify-webhooks-requests
- **[EM-14]** https://resend.com/docs/send-with-go
- **[EM-15]** https://www.brevo.com/pricing/ (fetched via curl; plan prices are injected client-side and were absent from the HTML)
- **[EM-16]** https://developers.brevo.com/docs/how-to-use-webhooks
- **[EM-17]** https://github.com/getbrevo/brevo-go
- **[EM-19]** https://www.twilio.com/en-us/products/email-api/pricing (https://sendgrid.com/en-us/pricing 301-redirects to https://www.twilio.com/en-us/sendgrid, which has no prices)
- **[EM-20]** https://www.twilio.com/docs/sendgrid/for-developers/tracking-events/getting-started-event-webhook-security-features
- **[EM-21]** https://github.com/sendgrid/sendgrid-go
- **[EM-22]** https://www.mailgun.com/pricing/
- **[EM-23]** https://github.com/mailgun/mailgun-go

