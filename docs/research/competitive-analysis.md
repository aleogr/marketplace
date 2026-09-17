# Competitive Analysis

> **Status:** research document supporting `docs/requirements.md`. It records what competitors do; it does not change any requirement. The last section lists suggestions for the owner to accept or reject.
>
> **Access date for every source:** 2026-09-17, unless a source line says otherwise. Each fact carries a reference such as `[ML-1]`; the reference lists in the [Sources](#sources) section give the URL and what the page supports.
>
> **Legend:**
> - *Official* means a page published by the competitor (help center, seller center, terms, fee page, official blog).
> - *(secondary)* marks facts taken from press, consultants or integrators because the official page was unreachable or does not publish the figure. Secondary figures are quoted, not endorsed.
> - `not found` means the data point was searched for and not located; nothing in this document is estimated.
> - Amounts are quoted in the competitor's currency (BRL for Brazilian sites, USD for eBay). Percentages apply to the base each competitor defines; see the observations in each section.

## Scope and method

Competitors covered:

| Code | Competitor | Segment | Why it is here |
|---|---|---|---|
| ML | Mercado Livre | Generalist marketplace + classifieds (vehicles, real estate) | Largest marketplace in Brazil; the reference for fees, reputation and logistics |
| OLX | OLX Brasil | Horizontal classifieds with optional escrow ("Garantia da OLX"); largest vehicle vertical | C2C model, free listings, buyer-pays fees |
| SHP | Shopee Brasil | Generalist, low ticket, CPF and CNPJ sellers | Price-band commission model, escrow, aggressive free shipping |
| AMZ | Amazon Brasil | Generalist catalogue marketplace | Category commission table, Buy Box, account-health metrics |
| MGL | Magalu marketplace | Generalist retailer + marketplace (CNPJ only) | Reputation-gated payouts, shared shipping cost |
| KBM | KaBuM! | Electronics niche (1P + 3P marketplace, Magalu group) | Direct reference for the electronics MVP |
| WM | Webmotors | Vehicle classifieds | Direct reference for the vehicles MVP |
| ALI | AliExpress | Cross-border generalist (with local Brazilian sellers) | International reference for the future generalist marketplace |
| EB | eBay | International generalist + eBay Motors | International reference for vehicles, reputation and fees |

Method: nine parallel research passes, one per competitor, reading official pages directly (help centers, seller centers, terms, fee pages, public help-center JSON APIs where the HTML is client-rendered). Several Brazilian sites block non-browser clients (HTTP 403 or captcha): Mercado Livre search pages and its fee landing page, OLX main site, Magalu search pages, Webmotors main site (read through a text proxy), Amazon Seller Central help (login-only). Where only secondary sources were reachable, they are marked. The AliExpress pass ran out of time; its findings are limited to what was confirmed and the rest is marked `not found`.

Numbering of dimensions follows the task brief: 1 fees, 2 seller onboarding, 3 payments and payouts, 4 shipping, 5 search and comparison, 6 reviews and reputation, 7 moderation, 8 vehicles, 9 languages. Section 10 lists the implications for `docs/requirements.md`.

---

## 1. Commissions, fees and paid placement

| | Commission per sale | Fixed / listing / subscription fees | Paid ads and highlights | Payment processing fee |
|---|---|---|---|---|
| **ML** | Free to list. Clássico 10–14% (no seller-funded installments); Premium 15–19% (up to 10x/12x interest-free). Exact electronics %: `not found` (table is client-rendered and bot-blocked). Vehicles: 0% [ML-1][ML-3][ML-4][ML-12] | Items < R$ 12,50: fixed cost = 50% of price; R$ 12,50–79: "three bands" (values `not found`); ≥ R$ 79: none. Store page "Minha página" R$ 99/month [ML-3][ML-4][ML-2] | Product Ads: CPC auction (AdScore × bid), daily budget; Display Ads: CPM. Typical CPC `not found`. Vehicles: paid tiers Prata/Ouro/Diamante (see §8) [ML-5][ML-6][ML-12] | None separate: the sale fee bundles Mercado Pago processing [ML-7] |
| **OLX** | 0% on sales through Garantia da OLX. Buyer pays a "fixed + variable" guarantee fee shown only at checkout (amounts `not found`) [OLX-2][OLX-3] | Free listing quotas per category (Autos 1 per 6 months, phones 5/month, most goods 20/month); extra listing priced at checkout (`not found`). Pro plans "from R$ 59,90" (per-plan prices `not found`) [OLX-7][OLX-8][OLX-10][OLX-11] | Fixed-price highlights per listing: Básico (1 day), Prata (7 days, 3 bumps), Ouro (5 bumps), Diamante (7 bumps + home gallery), Ouro Quinzenal (15 days), Diamante Mensal (30 days). Official prices `not found`; (secondary) R$ 20,99 / 25,99 / 33,99 [OLX-17][OLX-18][OLX-35] | Optional seller benefit: free shipping R$ 29,90 per sale (another official article: R$ 25,90–27,90); free shipping + 10x interest-free: R$ 29,90 + 5% of price [OLX-2][OLX-3][OLX-6] |
| **SHP** | Official fee page is client-rendered (`not found`). (secondary, valid from 1 Mar 2026) price bands, not categories: ≤ R$ 79,99: 20% + R$ 4/item; R$ 80–99,99: 14% + R$ 16; R$ 100–199,99: 14% + R$ 20; ≥ R$ 200: 14% + R$ 26; < R$ 8: 50% of price. CPF sellers: + R$ 3/item above 450 orders in 90 days [SHP-5][SHP-6][SHP-7][SHP-10] | No listing or subscription fee (secondary) [SHP-5]. Seller return fee R$ 10 when at fault (secondary) [SHP-34] | Shopee Ads: CPC auction, minimum credit R$ 15 (one FAQ snippet says R$ 50), credits non-refundable; cost-per-order option in GMV Max for items with > 10 orders/day. Typical CPC `not found` [SHP-11][SHP-12][SHP-14][SHP-15] | Terms mention a "Taxa de Transação" deducted before release; amount `not found` (secondary: embedded in the commission) [SHP-3][SHP-9] |
| **AMZ** | 10–15% by category on final price including shipping, minimum R$ 1 (R$ 2 in some categories). Electronics: TV/audio 10%, cell phones 11%, PC 12%, portable electronics 13%, accessories 15% up to R$ 100 then 10%. Table dated 20/01/2025 [AMZ-1] | Individual plan: R$ 2 per item sold, no monthly fee. Professional: R$ 19/month, free for 12 months for new sellers. Interest-free installment program: 1,5% of sales on enrolled products ≥ R$ 40 [AMZ-1][AMZ-4] | Sponsored Products / Brands: CPC, daily budget, Professional account and Buy Box eligibility required; R$ 250 in free clicks for new sellers. Typical CPC and minimum bid `not found` [AMZ-13][AMZ-15][AMZ-1] | None separate; seller receives the full amount on installment sales [AMZ-1][AMZ-4] |
| **MGL** | "Commission per sale + fixed cost per order", no monthly fee. Headline standard rate 18%; new sellers 9,9% for 3 months or R$ 100k; acceleration program 13% (advance payout) / 11% (installment flow). Official category table `not found`; (secondary) electronics 16–19% [MGL-1][MGL-2][MGL-3][MGL-28] | Fixed fee per order: official amount `not found`; (secondary) R$ 3–5 per item above R$ 10 [MGL-36] | Magalu Ads: sponsored products CPC, banners CPM; minimum top-up R$ 50; per-category minimum CPC (values `not found`); 15% fee on offsite (Meta/Google) transfers [MGL-10][MGL-11] | `not found` as a separate line; payout transfer fee R$ 5 waived with MagaluPay [MGL-4] |
| **KBM** | Flat 18% for all departments, "repasse à vista". (secondary) base is product + shipping [KBM-2][KBM-13] | No listing or monthly fee stated. Refund fee 6,5% on every refunded order, regardless of reason [KBM-2][KBM-25] | KaBuM! Ads: self-service retail media, auction pricing, prepaid credits; formats from banners to sponsored brand. Rates `not found` [KBM-20] | `not found` (payout via MagaluPay) |
| **WM** | 0%: "no fee or percentage on the vehicle transaction" [WM-1][WM-75] | Private sellers: mandatory paid plan Economic / Plus / Premium; no free ad; prices vary "by city, state, device and car" and are shown only behind login (`not found`). Dealers (Cockpit): Performance = monthly fee + per lead; Controle = subscription + lead franchise; Start = 30 ads, free leads for cars ≤ R$ 35.000; POP 05/10 slots for cars ≤ R$ 50.000. Prices `not found` [WM-2][WM-5][WM-27][WM-28][WM-29][WM-31] | Turbinar / +Fotos (private), Aceleradores / Super Acelerador Nitro / Visão 360º (dealers); prices `not found`. Free "Super Preço" badge when price is 1–15% below FIPE [WM-12][WM-42][WM-9] | Not applicable (no payment processing) |
| **ALI** | Research pass incomplete: `not found` | `not found` | `not found` | `not found` |
| **EB** | Final value fee 13,6% on most categories up to USD 7.500 + 2,35% above; per-order fee USD 0,30 (≤ USD 10) or 0,40. Store subscribers: electronics 9,35% up to USD 2.500. Vehicles: no final value fee. International fee 1,65%; currency conversion 3%; Below Standard sellers +6% [EB-1][EB-2] | 250 free listings/month, then USD 0,35. Stores: Starter USD 7,95, Basic 27,95, Premium 74,95, Anchor 349,95 per month. Dispute fee USD 20 [EB-1][EB-2] | Promoted Listings General: seller picks an ad rate of 2–100% of the sale, charged only on sale within 30 days of a click. Priority: CPC second-price auction, daily budget USD 1 to 1.000.000. Typical CPC `not found` [EB-9][EB-10] | Included in the final value fee [EB-1] |

**Observations**

- Two revenue models coexist: commission on completed sales for goods (ML, SHP, AMZ, MGL, KBM, EB) and pay-to-list or pay-per-lead for vehicles (ML vehicles, OLX plans, WM, EB Motors). No competitor charges a percentage on a vehicle sale.
- Commission bases differ: Amazon and eBay charge on the final price including shipping; KaBuM reportedly does the same (secondary); Mercado Livre states the fee is on the product price. OLX charges the buyer, not the seller.
- Fixed fees per item for low-priced goods are the norm (ML below R$ 79, Shopee R$ 4–26 per item, Magalu per order, Amazon Individual R$ 2). Shopee replaced category rates with price bands in March 2026.
- The interest-free installment cost is always an explicit, seller-paid add-on: ML Premium (+5 points over Clássico), Amazon 1,5%, OLX +5%, Magalu 13% vs 11% depending on payout mode.
- Paid placement is almost always a CPC auction with a daily budget; eBay adds a cost-per-sale model; OLX and ML vehicles sell fixed-price highlights with a duration and "bumps". No competitor publishes typical CPC prices.
- Fees are volatile: Shopee changed in March 2026, Magalu promotions are dated September 2026, Amazon's table is dated January 2025 with 2024 changes; any fee schedule the platform adopts needs versioning and effective dates.

## 2. Seller registration, verification and limits for new stores

| | Who can sell | Verification (KYC) | Bank account | Limits for new sellers |
|---|---|---|---|---|
| **ML** | Individuals (CPF) or companies (CNPJ); Mercado Pago requires residence in Brazil, so non-residents cannot receive payments [ML-10][ML-11][ML-15] | ID document scan + facial recognition; company representative submits corporate documents; validation "up to 72 hours" (search snippet) [ML-17] | Not required: proceeds land in the Mercado Pago account [ML-15][ML-18] | Free listings: max 10 simultaneous, stock 1, only while ≤ 5 sales/year (new) or ≤ 20 (used). No reputation colour until 10 completed sales in 365 days. "Programa Decola": R$ 250 deposit buys a green reputation for up to 365 days [ML-1][ML-2][ML-20][ML-14] |
| **OLX** | CPF or CNPJ, 18+, one account per person; foreigners without CPF/CNPJ: `not found` [OLX-22][OLX-23] | Biometric selfie + RG/CNH, approved within 24 h, gives "Identidade verificada" badge; wallet needs a regular CPF matching Receita Federal, activated in 48 business hours; checks against Receita, SPC/Serasa [OLX-25][OLX-28][OLX-36] | Withdrawal account must be in the same CPF at a homologated bank [OLX-28] | Same listing quotas as everyone; transactions R$ 10–20.000; no probation or new-seller payout hold found (`not found`); security block of funds up to 10 days on suspicion [OLX-29][OLX-36][OLX-37] |
| **SHP** | CPF (including MEI) or CNPJ; foreign companies only through the cross-border program (paid in USD) [SHP-17][SHP-3] | (secondary) CPF: RG/CNH; CNPJ: CNPJ card, social contract, administrator ID; official migration deck: shareholder ID + selfie, review up to 10 days [SHP-18][SHP-17] | Same name/CPF/CNPJ as the account, no savings or third-party accounts (secondary) [SHP-20] | Listing caps, sales caps, probation: `not found`. Listing quota may be limited for 28 days for sellers with penalty points [SHP-58] |
| **AMZ** | CPF or CNPJ with e-mail, bank account and credit card; foreign companies have a documents section (list `not found`) [AMZ-4][AMZ-6] | ID photo + selfie, optional video call; verification "usually up to two business days"; proof of address ≤ 180 days; periodic re-verification [AMZ-5][AMZ-6] | Required, plus a credit card (R$ 1 verification charge) [AMZ-4] | Listing/sales caps `not found`. All sellers: DD+7 delivery reserve; 3-day reserve after bank changes [AMZ-21][AMZ-22] |
| **MGL** | CNPJ active > 3 months, able to issue NF-e. CPF only for the affiliate ("Divulgador") program. Foreign sellers `not found` [MGL-1][MGL-24] | (secondary) CNPJ card, NF-e certificate, representative ID, state registration; approval within 48 h [MGL-29][MGL-36] | Account in the CNPJ; MagaluPay pushed as bank domicile [MGL-4] | Listing caps `not found`. Sellers without a calculated reputation are paid at "dispatch + 28 days" instead of + 3 [MGL-4] |
| **KBM** | CNPJ active ≥ 3 months, NF-e, new products with physical stock, no dropshipping; CPF not accepted; foreign sellers `not found` [KBM-2][KBM-33] | Pre-registration form, team contact within 1 business day (hotsite) or 5 (official blog); contract signed after approval; purchase invoices may be requested at any time [KBM-2][KBM-6][KBM-33] | Via MagaluPay account pre-registered by KaBuM [KBM-30] | Seller Score starts only after > 10 approved orders; listing caps or probation `not found` [KBM-22] |
| **WM** | Private individuals 18+; dealers (CNPJ) through Cockpit; 0 km cars only by registered stores; foreign sellers `not found` [WM-1][WM-6][WM-26] | Two-step login code; CPF + facial biometrics; CNH/RG manual check in specific cases (terms updated May 2026) [WM-7][WM-8][WM-26] | Not required (no payouts) [WM-14] | `not found`. Ad refund allowed at most twice in 12 months [WM-11] |
| **ALI** | `not found` | `not found` | `not found` | `not found` |
| **EB** | Residents of listed countries, Brazil included; individual or business accounts [EB-3] | Name, address, date of birth, SSN/EIN, ID upload; businesses add incorporation documents [EB-12] | Checking account in the account holder's name (US); Brazilian-seller requirements `not found` [EB-12][EB-13] | Selling limits exist and are reviewed monthly; the number is not published (secondary: ~10 items / USD 500 per month). New private sellers' funds held up to 31 days after payment until 10 sales and USD 150 [EB-14][EB-15][EB-16] |

**Observations**

- Every Brazilian competitor that processes payments verifies identity with a document plus a selfie or facial biometrics, in addition to the payment institution's own KYC. Verification badges ("Identidade verificada") are shown to buyers at OLX.
- Niche and curated marketplaces (KaBuM, Magalu) accept only companies with an active CNPJ and NF-e; generalists (ML, Shopee, Amazon, OLX) accept individuals.
- "Limits for new sellers" are rarely a listing cap. The common instrument is a longer payout hold until a track record exists (eBay 10 sales / USD 150; ML no reputation until 10 sales; Magalu + 28 days without reputation; KaBuM score after 10 orders), plus reduced exposure (ML free listings, orange/red sellers cannot win the Buy Box).
- Mercado Livre monetises the cold-start problem: a refundable R$ 250 deposit buys a provisional green reputation.

## 3. Payment methods, installments, split and payouts

| | Buyer payment methods | Installments | Split / collection model | Payout timing | Chargebacks and refunds |
|---|---|---|---|---|---|
| **ML** | Credit cards (up to 24x), debit, Mercado Crédito (12x), account balance, Pix, boleto (1–2 business days), Mercado Coin [ML-22] | Premium listings: interest-free up to 10x/12x from R$ 500, 18x for tech > R$ 600 with the Mercado Pago card; minimum installment R$ 5; Clássico: buyer pays interest [ML-1][ML-23] | ML/Mercado Pago collects, deducts sale fee and shipping cost, credits the seller's Mercado Pago account [ML-1][ML-26] | Mercado Envios: 8 days after delivery (new, with reputation), 12 days (used or no reputation). Own shipping: 5 days with buyer confirmation, 29 days otherwise. MercadoLíder: immediate / 11 / 28 days [ML-18][ML-24b] | Buyer protection refunds non-receipt (28 days) or defects (30 days new). Seller bears return cost only when the item is different/defective. Chargeback coverage via "Programa de Proteção ao Vendedor" (≥ 10 sales; no proof needed with Mercado Envios) [ML-25][ML-27] |
| **OLX** | Pix (5-minute window), credit cards (8 brands), NuPay, wallet balance; boleto not offered for purchases [OLX-38] | Card installments from R$ 30 per installment; buyer pays interest unless seller opts into 10x interest-free (R$ 29,90 + 5%); max installments with interest `not found`; seller always paid in full upfront [OLX-6][OLX-38][OLX-40] | Escrow: Carteira OLX (iFood Pago) holds the amount until the transaction concludes; one seller per order [OLX-29] | Released 48 h after delivery confirmation; bank transfer up to 2 business days; minimum withdrawal R$ 1 [OLX-29][OLX-40][OLX-41] | Mediation must be opened within 48 h of delivery; parties answer within 3 business days; refund conditioned on return; contestation decided by iFood Pago/OLX [OLX-29][OLX-42][OLX-36] |
| **SHP** | Credit card, Google Pay, Pix (1–24 h), boleto (3 business days), SParcelado (BNPL), Maree wallet, Caixa virtual debit; two cards can be combined [SHP-21][SHP-22] | Up to 12x, minimum R$ 5 per installment; seller may price by installment count; surcharge shown at checkout; R$ 60 minimum for no-surcharge installments; official interest-free count `not found` [SHP-24][SHP-25] | ShopeePay collects; funds "blocked" under Garantia Shopee until release [SHP-3] | Released when the buyer confirms receipt or after the 7-day withdrawal period; official example 3 + 7 + 7 = 17 days from order. Withdrawal to bank 3–5 business days (secondary) [SHP-27][SHP-29][SHP-30] | Chargebacks and returns offset against the seller balance; return shipping paid by seller only when at fault; refund to buyer: Pix ≤ 3 days, card ≤ 2 statements [SHP-27][SHP-31][SHP-33] |
| **AMZ** | Visa, Mastercard, Elo, Amex, Pix (30-minute window), NuPay, Livelo points, "Parcele sem cartão" (Geru, 3–24x); boleto no longer in the current list (status `not found`) [AMZ-16][AMZ-17][AMZ-27] | Seller FAQ: up to 10x interest-free; 2023 article: 12x with R$ 50 minimum installment; seller funds it through the 1,5% program [AMZ-4][AMZ-27][AMZ-1] | Amazon collects and pays the seller the full amount regardless of installments; sub-accounts `not found` [AMZ-4] | Every 14 days; reserve DD+7 after delivery; up to 5 business days for the deposit to land; account-level reserves for claims (Amazon staff forum posts) [AMZ-4][AMZ-21][AMZ-22] | A-to-Z claims (90-day window, 48 h seller response) and chargebacks count against the seller and are charged to seller-fulfilled orders [AMZ-19][AMZ-20][AMZ-29] |
| **MGL** | Pix (key valid 5 minutes), boleto, credit cards (7 brands), Caixa virtual debit, MagaluPay, Carnê Digital, two cards (app) [MGL-18][MGL-19] | Other cards up to 10x interest-free; own card up to 21x (2024 promotion). Who pays interest on marketplace items `not found` [MGL-19][MGL-34] | Magalu collects and "repasses" net of commission. Seller chooses "Antecipação Automática" (full amount, higher commission) or "Fluxo" (per installment, lower commission) [MGL-4][MGL-3] | With MagaluPay + reputation: Magalu Entregas "dispatch + 3 days", own shipping "delivery + 7 days"; otherwise "dispatch + 28 days" (table updated 14 Sep 2026) [MGL-4][MGL-5] | Seller-side chargeback rule `not found`; buyer terms make partner stores responsible for their offers; 7-day withdrawal [MGL-17] |
| **KBM** | Pix (discount "up to 10%" / "up to 15%" depending on page; QR valid 20 minutes), credit card, boleto, NuPay (up to 36x), Cartão KaBuM! (24x promotional) [KBM-34][KBM-37][KBM-4][KBM-35] | Observed 10x interest-free on a R$ 5.099,99 GPU (card price 10% above Pix price); who pays the interest `not found` [KBM-9] | KaBuM collects and pays via MagaluPay [KBM-24] | Cycle 1st–31st, paid on day 7 of the next month (help center) or 5th business day (hotsite). Order must have NF-e, tracking, "received = yes", no refund request and no open incident [KBM-24][KBM-2][KBM-23] | 6,5% refund fee to the seller on any refunded order; seller is still paid if fraud is suspected but payout rules were met; explicit chargeback allocation `not found` [KBM-25][KBM-45] |
| **WM** | Not applicable to the vehicle price. Ad plans: credit card up to 10x, Pix, boleto [WM-1][WM-6] | Santander financing embedded in the ad, up to 60 installments [WM-13] | None: Webmotors is an intermediary; no escrow (the 2018 "Autopago" wallet no longer resolves) [WM-12][WM-51] | P2P financing: Santander pays the seller within 2 business days after the transfer is confirmed [WM-14][WM-21] | Only for the ad fee (refund via card issuer, up to two statements) [WM-17] |
| **ALI** | `not found` | `not found` | `not found` | `not found` | `not found` |
| **EB** | Cards (no Amex), PayPal, Apple Pay, Google Pay, Venmo, Klarna; Escrow.com for vehicles [EB-17] | Klarna: Pay in 4 interest-free; 6–24 months at 7,99–35,99% APR paid by the buyer; USD 35–30.000 [EB-18] | eBay collects and deducts fees ("managed payments") [EB-1][EB-13] | 1–2 days to "available" + 1–3 business days to the bank; new private sellers held up to 31 days after payment (business sellers 15 days) [EB-13][EB-16] | Seller has 5 days to respond to a dispute; hold up to 90 days; USD 20 dispute fee unless seller protection applies (tracking shows delivery) [EB-19][EB-20][EB-1] |

**Observations**

- Pix and credit cards are universal in Brazil. Boleto is still offered by Shopee, Magalu, KaBuM and ML, not by OLX for purchases, and appears to have been dropped by Amazon. Buy-now-pay-later products (NuPay, SParcelado, Geru, Klarna) are common add-ons.
- Sellers are paid the full amount regardless of the buyer's installments (ML, OLX, Amazon, Magalu "Antecipação"); the cost of interest-free installments is a seller-side fee or a higher commission tier. Magalu is the only one letting the seller choose to receive per installment at a lower commission.
- Every marketplace holds the payout until after delivery: 48 h (OLX), 3–8 days (Magalu with reputation, ML with reputation), 7 days (Shopee, aligned with the legal right of withdrawal, Amazon DD+7), 28 days (Magalu without reputation, ML without buyer confirmation), 31 days (eBay new sellers). Hold length depends on reputation, shipping mode and delivery confirmation.
- Chargebacks are the seller's cost by default, softened by protection programs conditioned on using the platform's shipping label with tracking (ML, eBay).
- Refund fees on cancelled orders exist (KaBuM 6,5%, Shopee R$ 10 for seller-fault returns); ML refunds its sale fee only when the sale is cancelled.

## 4. Shipping: quotes, who pays, labels, tracking, pickup

| | Who quotes | Who pays / free shipping | Labels and carriers | Tracking, pickup and in-person |
|---|---|---|---|---|
| **ML** | Platform ("Envios no Mercado Livre") from billable weight = max(physical, L×W×H/6000) and product price; ML re-measures parcels [ML-28][ML-30] | New items R$ 19–78,99: buyer gets free shipping and ML pays the cost. ≥ R$ 79: seller must offer free shipping and pays the Envios table with up to 50–70% discount by reputation (example 0,3 kg: R$ 5,65 to R$ 21,65 by price band). Below R$ 19 or used: optional, seller pays [ML-28][ML-31][ML-32][ML-59] | One label per sale generated by the platform; modes Full (fulfillment), Flex (own couriers, same day), Coleta (pickup), Agências (drop-off), Correios and carriers; label for Coleta/Agências needs the NF-e key [ML-28][ML-39] | Tracking for both parties in the platform. Without Envios the buyer arranges delivery with the seller after purchase. Buyer pickup at lockers/agencies for deliveries: `not found` [ML-41] |
| **OLX** | Platform at checkout, from distance and the weight declared by the seller [OLX-40] | Buyer pays shipping and the guarantee fee; seller can buy the free-shipping benefit (R$ 29,90; Correios quotes ≤ R$ 30 become free) [OLX-5][OLX-8] | Label generated by OLX within 24 h with a DACE; Correios (≤ 30 kg), Loggi, Jadlog (≤ 20 kg), J&T, R3, 99 Entregas Flash (same day, ≤ 10 kg) [OLX-44][OLX-45][OLX-46] | Tracking in the sale details plus WhatsApp updates; "Clique Retire" lockers (72 h to collect); "Retirar com vendedor" in person, still escrowed [OLX-40][OLX-39] |
| **SHP** | Platform, from the dimensions and weight entered by the seller and the carriers' tables; where a Shopee logistics option is enabled the seller must use it [SHP-35][SHP-36] | Buyer pays unless a free-shipping coupon applies (covers up to R$ 20 of freight, max 10 coupons/week, app only). Seller-side program (secondary): mandatory since Mar 2026, subsidy caps R$ 20 / 30 / 40 by price band [SHP-37][SHP-38][SHP-7] | Label from the Seller Centre; Shopee Xpress pickup for selected sellers; Correios drop-off; cross-border parcels delivered by Correios [SHP-39][SHP-43][SHP-41] | In-app tracking; "Agência Shopee" pickup with a PIN; in-person handover `not found` (off-platform contact is banned) [SHP-42][SHP-40] |
| **AMZ** | Professional sellers set their own rates (shipping templates); Individual plan: Amazon sets the rate; DBA/FBA use Amazon's fee tables [AMZ-4][AMZ-1] | Free-shipping tiers: R$ 19–79 free with 3+ days, > R$ 79 free with 1 day (FBA); DBA per-unit fee R$ 4,50 / 6,50 / 6,75 below R$ 79, state table above; commission applies to item + shipping [AMZ-11][AMZ-8][AMZ-1] | DBA: seller prints the Amazon label; Total Express, Correios, Amazon Logistics; pickup at the seller (Mon–Fri) or drop-off points; DBA is the default for new sellers (≤ 22 kg, order ≤ R$ 20.000) [AMZ-8][AMZ-9][AMZ-12] | "100% trackable"; "Pontos de Retirada" with Jadlog (532 points at launch, 10-day hold); Amazon Locker in Brazil `not found`; seller-to-buyer in-person `not found` [AMZ-8][AMZ-30][AMZ-31] |
| **MGL** | Platform: after a sale Magalu Entregas "calculates freight, generates the label and sets the deadline" [MGL-13] | Cost shared between Magalu and the seller; sellers with > 97% on-time dispatch get up to 50% off their share on items over R$ 79. Buyer free shipping only on Magalu-sold items ≥ R$ 99 (furniture ≥ R$ 699), not in the North region, not for partners. Freight table `not found` (inside the seller portal) [MGL-13][MGL-16] | Label printed from the platform (common or Zebra printer); Fulfillment, Entrega Vapt (same day), Coleta e Entrega, Postagem na Agência (≤ 30 kg, ≤ 100 cm per side); Magalog network, partner carriers, Correios [MGL-13][MGL-14] | Seller must update tracking status within 1 business day per step; lockers and in-store pickup for partner items `not found` [MGL-6] |
| **KBM** | Third-party seller quotes through its own freight tables (CEP and weight ranges), Correios or an API integration [KBM-47][KBM-48] | Buyer pays the quoted freight; seller may set free shipping globally, per state or per offer; KaBuM subsidies for third-party sellers `not found`. First-party "Prime Ninja": fixed R$ 5,99 freight in São Paulo state, R$ 5 off elsewhere [KBM-52][KBM-53] | Seller ships and enters tracking (payout requires it); reverse label is entirely the seller's responsibility; "KaBuM! Entregas" label flow described only by an integrator (secondary) [KBM-23][KBM-58][KBM-51] | Buyer tracks in "Meus Pedidos"; Correios agency pickup for first-party orders (7 days); 3 delivery attempts; in-store pickup on the official help center `not found` [KBM-57][KBM-60] |
| **WM** | Not applicable: vehicles are handed over in person; "CarDelivery" home delivery is offered by partner stores [WM-15][WM-24] | — | — | — |
| **ALI** | `not found` | `not found` | `not found` | `not found` |
| **EB** | Seller sets flat, calculated, free or local pickup; eBay International Shipping quotes the international leg and import charges to the buyer [EB-22][EB-5] | Buyer pays the seller's stated shipping; under eBay International Shipping the buyer pays domestic, international and duties; free-shipping subsidies `not found` [EB-5] | eBay Labels with USPS, FedEx and UPS discounts (current percentages `not found`); eBay International Shipping to ~190 countries, Brazil in the table but a 2024 support reply says shipments to Brazil were restricted (conflicting) [EB-22][EB-5][EB-7] | Tracking uploads automatically from eBay labels; local pickup confirmed by a code the buyer brings; lockers `not found` [EB-23][EB-27] |

**Observations**

- Platform-generated labels with tracking are the norm in Brazil (ML, OLX, Shopee, Magalu, Amazon DBA). KaBuM's third-party sellers and eBay sellers arrange shipping themselves, but even there the platform requires tracking data before paying out.
- Free shipping is a subsidy game with published thresholds: R$ 19 and R$ 79 at ML and Amazon, R$ 99 at Magalu (first-party only), coupon caps at Shopee. The seller's share is discounted by reputation (ML up to 50–70%, Magalu up to 50% for > 97% on-time dispatch).
- Volumetric weight (L×W×H / 6000) is the billing rule at ML and Amazon, which is why weight and dimensions are mandatory listing fields.
- Pickup networks are widespread: agencies (ML, Magalu, Shopee), partner shops (Amazon with Jadlog), lockers (OLX with Clique Retire). In-person handover between buyer and seller exists only where the platform still escrows the payment (OLX) or where there is no payment at all (vehicles).

## 5. Search, filters and product comparison

| | Filters | Sorting | Product comparison | Catalog model |
|---|---|---|---|---|
| **ML** | UI filters `not found` (search pages blocked); the search API exposes "available_filters" and "available_sorts" [ML-34] | Relevance by default; price ascending/descending [ML-34] | `not found` | Hybrid: "Catálogo" product page where all listings of the same new product compete; the "vendedor em destaque" (Buy Box) is chosen on price, interest-free installments, shipping, reputation and stock; orange/red sellers cannot win; other offers under "Outras opções de compra"; non-catalog items stay one listing per seller [ML-35][ML-36] |
| **OLX** | Category, price, Garantia benefits, advertiser type, location (GPS, CEP, state, city); per-category attributes (car year/model, rooms) act as filters [OLX-48][OLX-50] | Relevance (default), most recent, highest price, lowest price [OLX-48] | `not found` | One listing per seller; duplicates removed [OLX-52] |
| **SHP** | Filter for "Vendedores Indicados"; (secondary) price range, brand, seller rating, location, free shipping; per-category attribute filters `not found` [SHP-47][SHP-48] | Relevância, Mais recente, Vendas Principais, Preço [SHP-46] | `not found` (only a "Similar" option in the cart) [SHP-49] | `not found` as an official statement; all articles refer to seller-specific product pages |
| **AMZ** | Attribute filters per category. Example for "smart tv 50 polegadas": brand, price, rating, condition, voltage, screen technology, screen type, OS, HDR format, screen size, resolution, HDMI ports, streaming services, Prime [AMZ-25] | Em destaque, price asc/desc, average customer rating, newest, best sellers [AMZ-25] | "Compare com itens semelhantes" widget on product pages; item count and attributes `not found` (client-rendered) [AMZ-24] | Single detail page per product; "Oferta em destaque" (Buy Box) chosen by price, availability and seller eligibility (ODR, cancellation, late shipment); other offers under "Outros vendedores" [AMZ-3][AMZ-24][AMZ-16b] |
| **MGL** | (search snippet only) category, brand, price, promotions, rating, "Vendido por", purchase type, product type; attribute filters `not found` [MGL-37] | (search snippet only) relevance, lowest/highest price, best sellers, best rated, newest [MGL-37] | `not found` | Single product page with a Buy Box; offers ranked by price plus reputation, delivery quality and product quality; sellers link their offer to an existing catalog product [MGL-23] |
| **KBM** | Per-category attribute filters (GPU example: memory, connections, RGB, low profile, series) plus "Vendido por KaBuM!", "Frete grátis", "OpenBox", "Prime Ninja", price, brands [KBM-8][KBM-10] | Price asc/desc, most rated, newest, most searched, manufacturer, promotions, relevance; 60/90/120 per page [KBM-8][KBM-10] | `not found` (no "Comparar" control in category, search or product HTML) [KBM-8] | Single product page per EAN ("Match por EAN"); Buy Box disputed on price and reputation; catalog curated before publication [KBM-2][KBM-61][KBM-70] |
| **WM** | ~15 filter groups: location, brand, model/version, year, new/used, seller type, price, "Abaixo da Fipe", mileage, Vistoriado, Visão 360°, transmission, fuel, ~30 equipment items, armouring, body type, colour, doors, documentation, plate ending, warranty/service history, CarDelivery, official stores, "Necessidades" presets [WM-23] | "Mais relevantes" by default; other options `not found` (client-rendered) [WM-23] | "Comparador de veículos": up to 4 models side by side, attribute-based, but only 0 km catalogue models [WM-25][WM-52] | One listing per seller; no Buy Box [WM-23] |
| **ALI** | `not found` | `not found` | `not found` | `not found` |
| **EB** | Item-specifics filters per category (brand, size, colour, type...); some specifics required, others recommended [EB-28] | `not found` (results page blocked) | "Item Compare": up to 3 listings without leaving results, electronics first (secondary, Feb 2026); official help page `not found` [EB-29] | Hybrid: one listing per seller by default; catalog products group listings on product pages; unique items stay separate [EB-30] |

**Observations**

- Attribute-based filters per category are confirmed at Amazon, KaBuM (electronics) and Webmotors (vehicles); they are the expected baseline in both MVP niches. Sort options converge on relevance, price both ways, best sellers, best rated and newest.
- Product comparison is rare: Amazon has a widget, Webmotors compares only new-car catalogue models, eBay is testing a 3-item compare. No Brazilian generalist offers a proper attribute comparison of arbitrary listings, which makes the requirement in section 8.3 a differentiator.
- The product/offer separation with a Buy Box is standard for new goods at ML, Amazon, Magalu and KaBuM (match by EAN/GTIN). Classifieds and eBay keep one listing per seller. Buy Box criteria are consistent: price, installments, shipping, reputation, stock.

## 6. Reviews and reputation

| | Product reviews | Store reputation | Buyer ratings | Q&A on listings |
|---|---|---|---|---|
| **ML** | Only buyers of the product on ML; 1–5 stars, text, photos and videos; attached to the catalog product; moderated before publication; one review per product; rewards (points, coupons) [ML-42][ML-43] | "Termômetro" red→green, gray until 10 sales in 365 days; window 60 days (≥ 101 sales) or 365. Thresholds: complaints green ≤ 2% / red > 8%; seller cancellations green ≤ 1,5% (60 d) or ≤ 2,5% (365 d); incorrect shipments green ≤ 10% / 13%. MercadoLíder from 230 sales and R$ 37.000 in 3 months; Gold 575 / R$ 118.400; Platinum 1.725 / R$ 296.000 [ML-20][ML-21][ML-44][ML-45] | Seller may rate the counterpart (positive/neutral/negative); "does not impact reputation in any way". Vehicles, real estate and services are outside the reputation system [ML-46][ML-47] | Public Q&A before purchase; sellers can block a buyer; contact data banned in questions and answers [ML-48][ML-49] |
| **OLX** | None (no catalog). Transaction ratings after a Garantia sale: 1–5 stars + up to 3 reasons, "em análise" for 15 days [OLX-54] | Automatic levels Básico → Intermediário → Avançado → Especialista from sales, verification, response speed, cancellations, complaints, ratings; badges "Identidade verificada", "Vistoriado", Google rating [OLX-56][OLX-25][OLX-57] | Sellers rate buyers (1–5, 15 days); not shown on the buyer's public page, visible to a seller inside sale details [OLX-55] | None; private chat with 90-day history [OLX-59][OLX-60] |
| **SHP** | Within 30 days of completion; 1–5 stars, text, photos, video; editable once; coins as reward; low ratings split into product / seller / logistics; sellers reply [SHP-50][SHP-51][SHP-52] | Store rating, response rate, shipping metrics; badges "Vendedor Indicado" and "Lojas Oficiais" (Shopee Mall). (secondary) Indicado thresholds: rating ≥ 4,5, response ≥ 60%, non-shipment ≤ 2%, ≤ 1 penalty point, CNPJ [SHP-47][SHP-53][SHP-54] | `not found` | None; private chat with a price-offer feature [SHP-56][SHP-57] |
| **AMZ** | Open to customers with ≥ R$ 50 spent in 12 months; "Compra verificada" label; AI-weighted star rating (recency, verified); photos/videos; moderated before publication; incentivised reviews banned [AMZ-32][AMZ-33][AMZ-34] | Seller feedback 1–5 within 90 days of the order; account-health metrics (see §7); public badges beyond the star average `not found` [AMZ-36][AMZ-3] | `not found` | "Perguntas" section with customer answers; sellers may answer [AMZ-24][AMZ-34] |
| **MGL** | Subject to Magalu approval, published in 2–4 business days; written from "Seus pedidos" (search snippet); verified-purchase rule and photos `not found` [MGL-22][MGL-39] | A calculated reputation score gates payout speed; SLA thresholds: > 95% delivered on time, < 5% complaints, < 1% external complaints, 98% of tickets answered within 2 business days; badges `not found` [MGL-4][MGL-6] | `not found` | Exists ("Comprou!" flag on answers; search snippet only) [MGL-38] |
| **KBM** | Star rating and count exist; verified-purchase requirement and photos `not found` [KBM-9] | Seller rating shown to buyers on the product page; internal "Seller Score" computed daily over 30 days after > 10 approved orders; only the < 2 suspension threshold is published [KBM-63][KBM-22][KBM-76] | `not found` | Q&A field exists per the site policies [KBM-3] |
| **WM** | Not applicable; seller reviews on listings `not found` | Badges only: Vistoriado (inspected, 120+ items), Super Preço, Loja Oficial, Loja Certificada (1-year warranty); star ratings `not found` [WM-9][WM-46][WM-56][WM-57] | `not found` | None; chat and proposals [WM-19] |
| **ALI** | `not found` | `not found` | `not found` | `not found` |
| **EB** | For catalog products only; 5-star average from purchasers; photos `not found` [EB-33] | Feedback +1/0/−1 and positive %, 60-day window; Detailed Seller Ratings on 4 criteria (1–5). Levels evaluated on the 20th: Above Standard defect ≤ 2%, cases ≤ 0,3%; Top Rated defect ≤ 0,5%, late shipment ≤ 3%, tracking ≥ 95%, ≥ 100 transactions and USD 1.000 in 12 months; Top Rated Plus gets 10% fee discount [EB-34][EB-35][EB-37][EB-11] | Sellers can only leave positive feedback for buyers; problem buyers are reported instead [EB-39] | "Contact seller" messaging with response-time badge; public Q&A `not found` [EB-40] |

**Observations**

- Verified purchase is the rule (ML, Shopee, eBay, Magalu via the order); Amazon allows unverified reviews but labels and down-weights them. Photos and videos in reviews are standard at ML, Shopee and Amazon. Reviews attach to the catalog product, not to a seller's listing (ML, eBay, Amazon).
- Reviews are moderated before publication (ML, Amazon, Magalu 2–4 days) with clear rejection reasons (contact data, links, unrelated content). Review windows exist (Shopee 30 days; Amazon seller feedback 90 days; eBay 60 days).
- Store reputation is quantitative and gates concrete outcomes: payout speed (Magalu, ML), Buy Box eligibility (ML, Amazon), fees (eBay +6% for Below Standard), exposure (ML free listings). Published thresholds cluster around complaints ≤ 1–2%, seller cancellations ≤ 1,5–2,5%, late dispatch ≤ 3–4%, tracking ≥ 95%.
- No competitor rates buyers publicly. OLX collects buyer ratings but hides them; eBay allows only positive feedback for buyers; ML's seller feedback has no effect. This matches the proposal in section 16 of the requirements.

## 7. Listing moderation, suspension and bans

| | Listing moderation | Prohibited items and wrong category | Suspension and ban rules | Appeal |
|---|---|---|---|---|
| **ML** | Mostly post-publication: listings cancelled or paused for review; preventive pauses on unusual price changes; seller notified by e-mail [ML-51][ML-54] | Prohibited-products policy with sanctions from cancellation to permanent selling suspension; wrong category is re-categorised automatically or cancelled; listings far from market price are paused (vehicles) [ML-52][ML-37][ML-19b] | Terms allow warning, suspension, restriction or deactivation; temporary suspension pauses listings but ongoing sales continue; permanent suspension cancels listings and unprinted-label sales; triggers include IP complaints, missing documents, links to another suspended account, unpaid invoices [ML-16][ML-54][ML-2] | IP claims need the rights holder's withdrawal; document issues are resolved by resubmitting [ML-54] |
| **OLX** | Pre-publication: "Aguardando publicação", live within 24 h or moved to inactive with a notice [OLX-61] | Long prohibited list (drugs, weapons, unregistered products, vehicles with debts...); refusal reasons include unrealistic price, keyword stuffing, wrong category (removed at OLX's discretion), duplicates [OLX-62][OLX-53] | Terms (Feb 2025): suspension or deletion for false data, misuse, illicit use, non-payment (after e-mail warning) or "any activity that, at OLX's discretion, is not in line with its policies"; wallet may be blocked without notice; guarantee terms terminated with 5 days' notice [OLX-64][OLX-36][OLX-29] | Formal process `not found`; support and ombudsman channels; reports handled within ~24 h [OLX-66] |
| **SHP** | Four violation types (prohibited, IP, spam, unsatisfactory quality); deleted listings add penalty points; heavy offenders limited to a 28-day listing quota; off-platform contact banned [SHP-58] | Prohibited-items policy exists but the page is client-rendered; wrong-category handling `not found` [SHP-59][SHP-60] | Terms: suspension, limitation or termination and retention of proceeds for fraud, multiple accounts, coupon abuse, harmful behaviour; penalty points cost benefits for 28 days (tier thresholds secondary) [SHP-3][SHP-62][SHP-63] | Via chat with photo ID, facial recognition and proof of address; must appeal within 30 days; response in 5 days [SHP-65] |
| **AMZ** | Open vs restricted categories (need authorisation); some products need approval before listing; automotive listings "submit and wait for approval" [AMZ-3][AMZ-19b] | Full restricted-products list is login-only (`not found`) [AMZ-43] | Account health: order-defect rate < 1% (60 days), late shipment < 4% (30 days), pre-shipment cancellation < 2,5% (7 days); DBA-specific thresholds < 2,5%; suspension notified in Seller Central and by e-mail [AMZ-3][AMZ-16b][AMZ-10] | "Plano de ação" (root cause, corrective and preventive actions); window in days `not found` [AMZ-16b] |
| **MGL** | Pre- vs post-publication `not found` | Prohibited list includes used and refurbished goods; "Inegociáveis" (no invoice, counterfeit, illicit origin, illegal goods) are grave faults leading to suspension; wrong-category handling `not found` [MGL-7][MGL-8] | SLA breach may lead to suspension or definitive termination; integrity pact allows immediate interruption [MGL-6][MGL-9] | `not found` |
| **KBM** | Pre-publication: products go through Aprovado / Alteração Necessária / Reprovado; offers cannot go live before approval; image and content rules [KBM-70][KBM-71][KBM-73] | Only technology and games categories; no used, refurbished or imitation goods; dropshipping → immediate suspension; "categoria não aceita" is a rejection cause; wrong-category article exists (details `not found`) [KBM-2][KBM-33][KBM-72][KBM-75] | Seller Score evaluated on days 1 and 15; score < 2 → 15-day suspension; brand-protection complaints inactivate the offer preventively and can end in permanent block ("Distrato") [KBM-76][KBM-77] | Only the brand-protection defence (purchase invoice or authorisation letter) is documented; general appeal `not found` [KBM-77] |
| **WM** | Pre-publication analysis "from a few seconds to 1 business day"; plan charged only after approval; ads with pending issues are not published [WM-59b][WM-60] | Description field auto-blocks contact data, location data, links and long digit sequences; terms forbid false or misleading content and identity misuse [WM-61][WM-26] | Suspension for founded suspicion of fraud, terms violation or court order, "at its sole discretion and without prior or later notice"; ads auto-deactivate after 90 days published + 30 days without visits [WM-26][WM-63] | `not found` |
| **ALI** | `not found` | `not found` | `not found` | `not found` |
| **EB** | Pre- vs post-publication `not found`; actions are removal, warning, restriction or suspension, notified by e-mail to seller and bidders [EB-41][EB-42] | 60+ prohibited/restricted policies; wrong-category official text `not found` (secondary: eBay investigates and moves the listing) [EB-41][EB-43] | Reasons: unpaid fees, owed refunds, unresolved buyer issues, policy violations, unverified data, account takeover; Below Standard sellers get lower ranking, lower limits, no ads, held funds and +6% fees [EB-44][EB-37] | Through Seller Help; formal timelines `not found` [EB-42][EB-44] |

**Observations**

- Curated and niche marketplaces moderate before publication (KaBuM, Webmotors, OLX); the largest generalist (ML) moderates after publication with automatic pauses. Pre-publication moderation is feasible at niche volumes and is what the two MVP niches' direct competitors do.
- Automatic detection of contact data in listing text is common (Webmotors blocks it in the description; ML demotes vehicle listings that contain it; Shopee bans off-platform contact). The masking proposal in section 15 of the requirements applies to listings as well as messages.
- Sanctions are graded: pause listing → limit quota → temporary suspension (KaBuM 15 days) → permanent ban, driven by measurable scores. Appeals with deadlines are documented only at Shopee (30 days to appeal, 5 days to answer) and Amazon (action plan).
- Several terms of use reserve the right to suspend "at sole discretion" and "without notice" (OLX, Webmotors). The requirements already go further (reason recorded, notice, appeal channel), which is also what the EU rules mentioned in section 20 demand.

## 8. Vehicle sales model

| | Listing model and price | Contact | Payment | Transfer, inspection, warranty |
|---|---|---|---|---|
| **ML** | Classifieds vertical, 0% commission. Private tiers (from 1 Nov 2024): Grátis (1 at a time, 60 days, low exposure); Prata R$ 79 cars / R$ 59 motorcycles / R$ 169 trucks; Ouro R$ 119 / 99 / 219; Diamante R$ 129 / 129 / 269. Dealer stock packages: prices `not found`. Plate mandatory (data pre-filled from it), up to 15 photos, optional YouTube video, AI-written description [ML-12][ML-19][ML-19b][ML-13] | Vehicles are "the only listing type where users may show contact information publicly": phone, WhatsApp, Q&A and messages; dealers receive leads, including pre-approved credit leads. Phone masking `not found` [ML-56][ML-57] | Platform does not process the vehicle price. "Reserva Online": buyer pays a reservation through Mercado Pago, held until the seller confirms delivery; balance settled between the parties. Financing simulator feeds partner lenders. Buyer protection excludes vehicles [ML-22][ML-58][ML-25] | `not found` (only buyer safety tips: check documents, fines, taxes) [ML-55] |
| **OLX** | Largest vertical (> 800k listings). Private sellers: 1 free ad per 6 months per vehicle subcategory, then paid ad or a Veículos plan "from R$ 59,90"; plate mandatory, up to 20 photos, make/model/plate editable for 3 days; RENAVAM for the vehicle history report; dealers get lead tools, integrators and Autoshift exposure [OLX-7][OLX-8][OLX-61][OLX-13][OLX-33] | OLX chat; phone hidden for private ads; professional ads may show phone/WhatsApp and each click generates a lead [OLX-59][OLX-60] | No payment processing or escrow (Autos excluded from Garantia). Financing simulator: Santander for private sales (cars ≤ 20 years), Safra for dealers; the bank pays the seller after documents are regularised [OLX-4][OLX-71] | Histórico Veicular report (fines, liens, theft, mileage; 72 h); "Vistoriado" badge after inspection by Detran-accredited partners, valid 120 days; transfer done by the parties at DETRAN, guidance only [OLX-69][OLX-57][OLX-74] |
| **SHP** | Not applicable: no vehicle category; automotive covers parts and accessories [SHP-1][SHP-61] | — | — | — |
| **AMZ** | Not applicable: automotive store lists parts, accessories and tyres only [AMZ-26][AMZ-19b] | — | — | — |
| **MGL** | Not applicable: no vehicle category; only vehicle consortium plans sold outside the marketplace [MGL-25] | — | — | — |
| **KBM** | Not applicable: technology and games only; e-bikes and scooters are sold as electronics [KBM-2][KBM-11] | — | — | — |
| **WM** | Vehicles only. Private sellers: mandatory paid plan Economic / Plus / Premium (prices vary by city, state, device and car; `not found`), no free ad, active until sold. Dealers: Performance (monthly + per lead, lead price varies with car price), Controle (subscription + lead franchise), Start (30 ads, free leads ≤ R$ 35.000), POP (cars ≤ R$ 50.000). Required: brand/model/version/year, ≥ 100 km, price checked against market, photos JPEG 540×420; plate for the Vistoriado badge; price 1–15% below FIPE earns a free "Super Preço" badge [WM-1][WM-2][WM-27][WM-28][WM-29][WM-31][WM-9] | In-platform chat is the primary channel; seller may opt to show a phone; dealers get leads (calls, proposals, financing simulations) with call masking, a lead limiter and regional display; WhatsApp CRM integration [WM-19][WM-6][WM-27][WM-66][WM-68] | No payment processing, no escrow (2018 "Autopago" wallet appears discontinued). Santander financing embedded in ads, up to 60 installments; person-to-person financing pays the seller within 2 business days after transfer; down payment settled directly [WM-12][WM-13][WM-14][WM-20][WM-51] | Guidance only: pay debts, notarise the CRV, buyer transfers within 30 days; vehicle debts payable in 12x via a partner; "Vistoriado" cautionary report (120+ items) uploaded by dealers; "Loja Certificada" dealers give a 1-year warranty [WM-71][WM-46][WM-57] |
| **ALI** | `not found` | `not found` | `not found` | `not found` |
| **EB** | eBay Motors. Listing fee instead of commission: USD 34 (≤ USD 15.000) or USD 79 (above); packages Basic USD 19 / Plus 34 / Premium 79 (photos, duration, reserve limits); dealer subscriptions (prices `not found`). VIN drives a free AutoCheck history report; branded titles must be disclosed [EB-8][EB-46] | eBay messaging; phone masking `not found` [EB-40] | eBay processes only a deposit (≥ 1% of price, 2,8% deposit processing fee); balance paid off-platform, via Escrow.com (2-day inspection period) or through "eBay Secure Purchase" (Caramel: full payment, financing, title transfer; USD 25 buyer fee). Vehicle Purchase Protection up to USD 500.000 (from Aug 2026); vehicles excluded from the Money Back Guarantee [EB-8][EB-47][EB-48][EB-49][EB-50] | Title transfer per state DMV rules, guidance only, except inside Secure Purchase where Caramel handles the title; inspection service `not found` [EB-47][EB-49] |

**Observations**

- No Brazilian competitor processes the price of a vehicle. The closest models are a reservation held by the platform until delivery (ML "Reserva Online") and bank financing embedded in the listing (Webmotors and OLX with Santander/Safra, which pays the seller after the transfer). eBay processes a deposit of at least 1% and, since 2026, offers a full in-flow purchase through a subsidiary.
- Revenue for vehicles comes from listing tiers with exposure levels and durations (ML, OLX), mandatory paid plans (Webmotors) and per-lead pricing for dealers (Webmotors). Commission on the sale is 0% everywhere.
- Plate is a mandatory field in Brazil (ML, OLX, Webmotors for badges) and is used to pre-fill vehicle data; FIPE is the price reference (ML links it; Webmotors badges prices 1–15% below it; OLX classifies by FIPE class). Listings far from market price are paused (ML) or rejected (OLX).
- Contact is deliberately more open for vehicles: ML allows public phone numbers only in this vertical; Webmotors and OLX push chat but let dealers use phone and WhatsApp with call masking and lead tracking.
- Trust services are add-ons sold around the listing: vehicle history reports, inspection badges valid for 120 days, dealer warranties, debt payment partners. Ownership transfer stays with the parties everywhere except eBay's Secure Purchase.

## 9. Languages and internationalization

| | UI languages | Currency | Country / language selection | Cross-border |
|---|---|---|---|---|
| **ML** | Portuguese only on mercadolivre.com.br (`lang="pt-BR"`, no selector); English versions of a few legal documents [ML-1b][ML-25] | BRL only [ML-56] | One domain per country with its own site id and currency (mercadolibre.com.ar, .com.mx...); geo-detection or user setting `not found` [ML-1b] | Buying: "Compra Internacional", buyer pays product + shipping + estimated import taxes at checkout; selling abroad from Brazil `not found` [ML-33] |
| **OLX** | Portuguese only; ads must be written in Portuguese [OLX-52][OLX-1] | BRL only | Standalone Brazilian JV; separate from other OLX-brand sites; location chosen by state/city/CEP or GPS [OLX-30][OLX-48] | None: items must be located in Brazil [OLX-52] |
| **SHP** | Portuguese and English, chosen in the app settings [SHP-67] | BRL; international sellers list in BRL and are paid in USD [SHP-3] | Separate domain per country (shopee.com.br); language is a user setting [SHP-3] | "Envio do exterior" items; import tax at checkout under Remessa Conforme: 20% up to USD 50, 60% minus USD 20 above, plus ICMS 17–20% (since 27 Jul 2024); selling abroad `not found` [SHP-68][SHP-69] |
| **AMZ** | Portuguese only (`lang="pt-br"`); no language selector found; the pt-BR "change language" help page does not exist [AMZ-24][AMZ-37] | BRL only; imported items priced with taxes included [AMZ-38] | One storefront per country chosen by domain; Seller Central has a language menu [AMZ-24][AMZ-21] | Buying from the US and China with import tax at checkout (60% + ICMS above USD 50); Brazilian sellers can sell on Amazon US without a US company [AMZ-38][AMZ-39][AMZ-40] |
| **MGL** | Portuguese only; selector `not found` | BRL only | Single site (plus sister brands); no per-country sites | Buying: imported items via the AliExpress partnership (Remessa Conforme), 6-business-day international dispatch; selling abroad `not found` [MGL-20][MGL-21][MGL-35] |
| **KBM** | Portuguese only (`lang="pt-br"`); Libras (sign language) translator on the home page [KBM-9][KBM-12] | BRL only [KBM-9] | No switcher [KBM-12] | Delivery to Brazilian addresses only; foreign sellers cannot register (CNPJ + NF-e required) [KBM-1][KBM-2] |
| **WM** | Portuguese only; the English help-center path serves Portuguese content [WM-73] | BRL only | Single-country site; jurisdiction São Paulo [WM-26] | None; parent CAR Group runs separate brands per country [WM-44] |
| **ALI** | `not found` | `not found` | `not found` | `not found` |
| **EB** | ebay.com in English only (a Portuguese URL slug renders in English); a pt-BR landing page exists for eBay International Shipping [EB-39][EB-6] | Listing currency is the site's; other sites show an approximate conversion; seller-side conversion charge 3% (US) or 2,5–3,5% by region (Latin America 3,5%) [EB-52][EB-1][EB-4] | One site per country (14+ country fee pages); no Brazilian site (pages.ebay.com/br returns 404); buyer "Site Preferences" for country, currency and units; geo-detection `not found` [EB-1][EB-32][EB-52] | Sellers ship internationally or list on other country sites; international fee 1,65% unless using eBay International Shipping; Brazilian sellers may register (1,55% international fee) [EB-53][EB-1][EB-3][EB-4] |

**Observations**

- Every Brazilian competitor is Portuguese-only with BRL-only pricing; Shopee is the exception with an English UI as a user setting. A two-language platform with the language in the URL has no local precedent among these nine.
- International players use one domain per country, each with its own language, currency and fee schedule (ML, Amazon, eBay). Language is a consequence of the country site, not an independent choice.
- Cross-border buying is handled by charging the buyer's estimated import taxes at checkout (ML, Shopee, Amazon), which became standard after Remessa Conforme. Cross-border selling from Brazil is documented only by Amazon (selling on the US site) and eBay (fees by seller region).

---

## 10. Implications for the requirements

Each item names the section of `docs/requirements.md` it affects and whether the analysis suggests **changing** a decision, **confirming** a proposal or open topic, or **adding** something the document does not cover yet. Nothing here is decided; the owner decides.

### 10.1 Business model and fees

1. **Add (§4, §23, §29):** the commission model needs more dimensions than "a commission on each sale". Competitors combine a percentage by category (Amazon 10–15%, ML 10–19%), a fixed fee per item or per order for low-priced goods (ML below R$ 79, Shopee R$ 4–26, Magalu, Amazon R$ 2), a minimum commission (Amazon R$ 1) and price bands (Shopee). Suggestion: model fees as a versioned schedule per marketplace with effective dates, supporting percentage by category, fixed fee per item, minimum commission and price bands, and make it a console parameter.
2. **Add (§29, commission base):** the open topic "commission before or after discounts" should also decide whether the base includes the shipping charged to the buyer. Amazon and eBay charge on the final price including shipping; ML on the product price.
3. **Add (§4, §29 installments):** interest-free installments are a seller-funded, opt-in feature everywhere (ML Premium tier, Amazon 1,5%, OLX +5%, Magalu 13% vs 11%), and the seller is paid in full upfront. Suggestion: treat "interest-free installments" as a per-listing or per-store option with an explicit fee, and let the fee schedule express it; this answers part of the open topic on who pays the interest.
4. **Change (§4, §2.3):** for the vehicles marketplace, no competitor charges a commission; revenue comes from paid listing tiers with exposure levels and durations (ML, OLX), mandatory plans (Webmotors) and per-lead pricing for dealers. Suggestion: state in §4 that the revenue model is defined per marketplace, and that vehicles may launch with paid listings or featured placement instead of commission, which moves that item out of "later phases" for that marketplace.
5. **Add (§2.3):** paid placement models seen: CPC auction with a daily budget (ML, Shopee, Amazon, Magalu, KaBuM), cost-per-sale as a percentage chosen by the seller (eBay, 2–100%), and fixed-price highlights per listing with a duration and repeated "bumps" (OLX, ML vehicles). The fixed-price highlight is the simplest to build and fits vehicles; note the options so the data model does not assume CPC.
6. **Add (§14, §4):** decide whether the commission is refunded on cancellation or return, and whether a cancellation fee exists. ML refunds the fee only when the sale is cancelled; KaBuM charges 6,5% on every refunded order; Shopee charges R$ 10 for seller-fault returns.
7. **Add (§2.3, optional):** store subscriptions are a third revenue stream at ML ("Minha página" R$ 99/month), Amazon (Professional R$ 19/month) and eBay Stores. Not needed for the MVP; worth listing under later phases.

### 10.2 Payments and payouts

8. **Confirm (§11, proposed payout hold):** every competitor holds the payout until after delivery: 48 h after confirmation (OLX), dispatch + 3 days (Magalu with reputation), 8 days after delivery (ML with reputation), delivery + 7 days aligned with the right of withdrawal (Shopee, Amazon DD+7), up to 28–31 days for stores without reputation (Magalu, ML, eBay). Suggestion: confirm the proposal and add that the hold length is a parameter driven by store reputation, shipping mode (platform label with tracking vs own shipping) and delivery confirmation.
9. **Confirm (§5):** PIX and credit card cover what every competitor offers. Boleto is still common (ML, Shopee, Magalu, KaBuM) but absent at OLX and apparently dropped by Amazon; buy-now-pay-later products (NuPay, SParcelado, Geru, Klarna) are add-ons. Keep boleto out of the MVP unless the gateway makes it free.
10. **Add (§14 disputes, §11):** define chargeback allocation. Competitors charge the seller by default and protect sellers who ship with the platform's tracked label (ML, eBay); eBay charges a USD 20 dispute fee. The requirements mention disputes but not who bears chargebacks.
11. **Add (§13):** a payment-method-conditioned discount ("Pix discount": KaBuM up to 10–15%, Shopee, secondary reports for Magalu) is a common promotion type. If it is wanted, the coupon/discount model should support a condition on payment method, and the commission base decision must say whether it applies before or after that discount.

### 10.3 Shipping

12. **Confirm and narrow (§12, §29 shipping scope):** platform-generated labels with tracking are the norm in Brazil (ML, OLX, Shopee, Magalu, Amazon DBA), and even where the seller ships (KaBuM, eBay) the platform requires tracking before paying out. Suggestion: the MVP includes label generation and tracking through the shipping provider, not quotes only; tracking is also the trigger for the payout hold.
13. **Add (§8.2, §12):** billable weight is max(physical weight, L×W×H / 6000) at ML and Amazon. The requirement for weight and dimensions is right; add that the shipping integration computes volumetric weight and that listings can be re-measured.
14. **Add (§12, §13):** free-shipping rules are threshold-based and subsidised (ML and Amazon at R$ 19 and R$ 79, Magalu at R$ 99, Shopee coupon caps). Suggestion: model "free shipping above a threshold" as a rule at marketplace or store level, with a defined payer (store, platform or shared), separate from coupons.
15. **Confirm (§12 future needs):** pickup points, agencies and lockers are widespread (ML agencies, Shopee agencies with PIN, Amazon partner shops with a 10-day hold, OLX lockers with 72 h). Keep pickup as a later phase but reserve a "pickup point" delivery type in the order model.

### 10.4 Catalog, search and comparison

16. **Confirm and detail (§8.2, §29 product/offer):** the product/offer separation with a Buy Box is standard for new goods at ML, Amazon, Magalu and KaBuM, which matches offers to catalog products by EAN/GTIN. Buy Box criteria are consistent: price, installments, shipping, reputation, stock. Suggestion: the electronics MVP implements the separation with GTIN matching and a Buy Box; vehicles stay one listing per store. Record the Buy Box criteria as an open design item.
17. **Confirm (§8.3):** attribute-based comparison of arbitrary listings does not exist at any Brazilian generalist; Amazon has a widget, Webmotors compares only new-car catalogue models, eBay is testing 3 items. It is a differentiator; suggest a limit of 3 to 4 products.
18. **Add (§9):** the standard sort options are relevance, price ascending and descending, best sellers, best rated and newest (Amazon, KaBuM, Magalu, Shopee). List them so the search design accounts for the aggregates they need (sales count, rating average).
19. **Add (§8.1, vehicles):** vehicle attributes observed as mandatory or as filters: plate (mandatory at ML and OLX, used to pre-fill data), mileage (≥ 100 km at Webmotors), fuel, transmission, body type, colour, doors, documentation status, plate ending, around 30 equipment items, and a price reference (FIPE at ML and Webmotors, with a free badge when 1–15% below it). ML pauses and OLX rejects listings far from market price. Suggestion: add plate lookup, FIPE reference and a price-anomaly check to the vehicles category design.
20. **Confirm (§8.4):** photo limits in use: ML vehicles 15 photos (min 800×600), OLX 20 for vehicles and 6 for goods, eBay 20–40 by package, KaBuM 500×500 white background. Video as an external YouTube link exists at ML vehicles. The configurable limits already required are enough; these are reference values.

### 10.5 Reviews, reputation and trust

21. **Confirm and extend (§16 product reviews):** verified purchase is the rule (ML, Shopee, eBay, Magalu). Suggestion to add: photos and videos in reviews (ML, Shopee, Amazon), a review window after delivery (Shopee 30 days), moderation before publication with explicit rejection reasons (ML, Amazon, Magalu 2–4 days), reviews attached to the product and shared by all offers (ML, eBay, Amazon), and a public reply by the store (Shopee). Rewards for reviews (ML coupons, Shopee coins) are an option to note, not to adopt now.
22. **Confirm (§16 buyer reviews, proposed):** no competitor shows buyer ratings publicly (OLX hides them, eBay allows only positive feedback, ML's have no effect). The proposal that buyers are not rated publicly matches the market.
23. **Add (§16, §20, §23):** store reputation is quantitative everywhere and gates concrete outcomes. Published thresholds: complaints ≤ 2% green at ML; seller cancellations ≤ 1,5% (ML) / < 2,5% (Amazon); late dispatch ≤ 3% (eBay Top Rated) / < 4% (Amazon); order-defect rate < 1% (Amazon); valid tracking ≥ 95% (eBay, Amazon); replies within 2 business days (Magalu, KaBuM). Outcomes: payout speed (Magalu, ML), Buy Box eligibility (ML, Amazon), fee surcharge (eBay +6%), exposure. Suggestion: define the reputation metrics, windows (60 or 365 days as at ML) and consequences as console parameters, and require a minimum number of completed sales (10 at ML and KaBuM) before a score is shown.
24. **Add (§20, limits for new stores):** the instruments competitors actually use are a longer payout hold until N completed sales (eBay 10 sales and USD 150; Magalu and ML by reputation), caps on simultaneous listings (ML 10 free listings) and exclusion from the Buy Box. Suggestion: make "new store" a state that ends after N completed sales without complaints, with those three limits as parameters.
25. **Add (§18, §20):** identity verification with a document plus selfie or facial biometrics is done by the platform itself at ML, OLX, Amazon and Webmotors, in addition to the gateway's KYC, and OLX shows a verified-identity badge. The requirement delegates KYC to the gateway (§11); consider whether a platform-level identity check and badge are needed, especially for vehicles where the platform does not process the payment.

### 10.6 Moderation and messaging

26. **Confirm (§8.5, §29 moderation):** the direct competitors of both niches moderate before publication (KaBuM approves products before offers go live; Webmotors analyses every ad in seconds to 1 business day; OLX within 24 h). ML moderates after publication with automatic pauses. Suggestion: pre-publication moderation for third-party listings in the MVP, with automatic pre-checks (category fit, contact data, price anomaly) and a manual queue.
27. **Extend (§15, proposed masking):** automatic detection of contact data applies to listing text too, not only to messages (Webmotors blocks it in descriptions; ML demotes; Shopee bans off-platform contact). Suggestion: extend the proposal to listings, Q&A and reviews.
28. **Decide (§15, §10 vehicles):** competitors deliberately allow public phone and WhatsApp for vehicles (ML only in this vertical; Webmotors and OLX for dealers with call masking and lead tracking) because the transaction happens off-platform. If the vehicles checkout keeps a reservation or deposit on the platform, masking can stay; if the platform only connects the parties, masking will be bypassed. This is a consequence of the vehicles checkout open topic.
29. **Add (§20):** appeals with deadlines are documented at Shopee (appeal within 30 days, answer in 5 days) and Amazon (action plan). Suggestion: set a response SLA for appeals as a parameter. Also note that graded sanctions (pause listing, limit quota, temporary suspension of 15 days at KaBuM, permanent ban) are driven by scores, which fits the layered-prevention approach.
30. **Add (§15):** Q&A on listings exists at ML, Amazon and Magalu and is public and moderated; Shopee, OLX and Webmotors use private chat only, and Shopee supports price offers in chat. The requirements cover messaging but not public Q&A; decide whether the MVP has it.

### 10.7 Vehicles

31. **Confirm and narrow (§10, §29 vehicles checkout):** no Brazilian competitor processes the vehicle price; the models are a reservation held by the platform until delivery (ML "Reserva Online"), a deposit of at least 1% with the balance off-platform (eBay), and financing embedded in the listing with the bank paying the seller after transfer (Webmotors, OLX). Suggestion: MVP = reservation or deposit paid through the platform and held until the parties confirm the handover, with the balance settled outside; full in-platform payment as a later option. The reservation must interact with the "unique item reserved during checkout" rule.
32. **Add (§29 right of withdrawal for vehicles):** ML excludes vehicles from its buyer protection and motorcycles from returns; eBay excludes vehicles from its Money Back Guarantee and sells a separate purchase protection. This supports treating vehicles under a separate protection policy, subject to legal review.
33. **Add (§28, §25 later):** trust services sold around vehicle listings are vehicle history reports (OLX, eBay AutoCheck), inspection badges valid 120 days (OLX, Webmotors), dealer warranties (Webmotors) and financing partners with pre-approved credit leads (ML, Webmotors, OLX). These are integrations for later phases; the listing model should be able to carry badges with an expiry date.
34. **Add (§8.2, dealers):** dealers are a distinct seller type everywhere (ML packages, OLX plans, Webmotors Cockpit with lead pricing, eBay dealer subscriptions) with stock integrators and lead management. The requirements treat all third-party sellers as "stores"; consider a dealer profile with different limits and pricing for the vehicles marketplace.

### 10.8 Internationalization and markets

35. **Confirm (§6):** every Brazilian competitor is Portuguese-only; Shopee alone offers English as a user setting. Two fully working languages with the language in the URL is a real differentiator and has no local precedent to copy.
36. **Add (§5, §29 cross-border):** cross-border buying is implemented by charging the buyer's estimated import taxes at checkout (ML, Shopee, Amazon under Remessa Conforme: 20% up to USD 50, 60% minus USD 20 above, plus ICMS). When cross-border sales are mapped, the order model needs an import-tax line and the rule that the buyer is the importer.
37. **Confirm (§6, §7):** international players choose the country by domain, one site per country, and language follows the country. The platform's model (market as an explicit concept, language independent of it) is different and more flexible; keep it, but note that per-country fee schedules and payout currencies are also per-site at ML, Amazon and eBay, which supports the "market" grouping in §5.

### 10.9 Gaps in this analysis

- AliExpress: the research pass did not complete; every AliExpress cell is `not found` and should be filled in a follow-up session.
- Exact per-category commission tables for Mercado Livre and Magalu, Shopee's official fee text, Webmotors and OLX plan prices, and typical CPC prices are not public or are behind login; they are marked `not found` rather than estimated.
- Search-page filters at Mercado Livre and Magalu could not be observed because the pages block non-browser clients.

---

## Sources

All sources were accessed on 2026-09-17. Entries marked *(secondary)* or *(search snippet only)* are not official pages of the competitor.

### Mercado Livre (ML)

- **[ML-1]** https://www.mercadolivre.com.br/ajuda/quanto-custa-vender-um-produto_1338 (also /ajuda/custos-de-vender-um-produto_870) — "Quanto custa vender um produto?": fee composition, Clássico 10-14% / Premium 15-19%, free-listing conditions, reduced fee R$150-700.
- **[ML-1b]** https://www.mercadolivre.com.br/ (homepage HTML) — lang="pt-BR", footer list of country sites.
- **[ML-2]** https://www.mercadolivre.com.br/ajuda/tarifas-e-faturamento_1472 (also /ajuda/Tarifas-y-facturacion_1044) — listing types table, Minha página R$ 99/month, fee refunded only on cancellation, invoice non-payment -> suspension.
- **[ML-3]** https://vendedores.mercadolivre.com.br/nota/como-funcionam-as-taxas-do-mercado-livre — seller center: fixed cost 50% under R$ 12,50, three bands to R$ 79, none above; reduced categories.
- **[ML-4]** https://developers.mercadolivre.com.br/pt_br/comissao-por-vender — developer docs (updated 03/09/2026): fixed_fee logic by logistic type/TH, Brazil activation 02/03, MLB percentage variability, logistic types.
- **[ML-5]** https://www.mercadolivre.com.br/ajuda/996 — Mercado Ads terms: CPC auction, AdScore, daily budget, CPM for display, invoicing terms.
- **[ML-6]** https://vendedores.mercadolivre.com.br/nota/o-que-e-e-como-funciona-o-product-ads — Product Ads pay-per-click, +26% sales claim.
- **[ML-7]** https://www.mercadopago.com.br/ajuda/33400 — no Mercado Pago processing fee on Mercado Livre sales.
- **[ML-10]** https://www.mercadolivre.com.br/ajuda/Como-empresa_4861 — switching personal account to company account.
- **[ML-11]** https://vendedores.mercadolivre.com.br/nota/quais-documentos-eu-preciso-para-operar-como-empresa — CPF vs CNPJ selling, documents, NF-e requirements.
- **[ML-12]** https://www.mercadolivre.com.br/ajuda/quanto-custa-vender-um-veiculo_868 — vehicle listing prices (valid from 1 Nov 2024): Diamante/Ouro/Prata/Grátis.
- **[ML-13]** https://www.mercadolivre.com.br/l/vender-carro — "Venda seu carro grátis": 0% commission, plate pre-fill, AI description, verification, Mercado Pago payment.
- **[ML-14]** https://www.mercadolivre.com.br/ajuda/33314 (and terms /ajuda/33285) — Programa Decola: R$ 250 deposit, 365 days, 5 affected sales, refund scale, R$ 250 ads credit.
- **[ML-15]** https://www.mercadopago.com.br/ajuda/termos-e-condicoes_300 (rendered via https://www.mercadolivre.com.br/ajuda/termos-e-condicoes_299) — Mercado Pago terms: Brazil residents with CPF/CNPJ only, KYC data sources, chargeback index de-accreditation, receivables domicile.
- **[ML-16]** https://www.mercadolivre.com.br/ajuda/Termos-e-condicoes-gerais-de-uso_1409 — Mercado Livre general terms: capacity (13+ with representative), registration, sanctions (sec. 7), IP.
- **[ML-17]** https://www.mercadolivre.com.br/ajuda/33107 — identity validation (document + facial recognition; company representative documents).
- **[ML-18]** https://www.mercadolivre.com.br/ajuda/275 — payout timing tables (Envios 8/12 days; own shipping 5/11/28-29 days; Mercado Shops 2/6).
- **[ML-19]** https://www.mercadolivre.com.br/ajuda/Custos-para-concessionarias_903 — dealer packages, renewal/cancellation rules.
- **[ML-19b]** https://vendedores.mercadolivre.com.br/nota/o-que-considerar-na-hora-de-anunciar-veiculos — vehicle listing requirements: plate format, 15 photos, contact phone, no contact data in description, price pausing.
- **[ML-20]** https://www.mercadolivre.com.br/ajuda/como-funciona-a-reputacao-como-vendedor_1382 — reputation basics, 10 first sales, 60/365-day windows, Decola.
- **[ML-21]** https://www.mercadolivre.com.br/ajuda/variaveis-reputacao_30193 — reputation variables (complaints, cancellations, incorrect shipments, mediations).
- **[ML-22]** https://www.mercadolivre.com.br/ajuda/Como-pagar-a-sua-compra_923 — buyer payment methods, up to 24x, Pix, boleto, vehicle reservations via Mercado Pago.
- **[ML-23]** https://www.mercadolivre.com.br/ajuda/22737 — Premium interest-free installment table, 18x tech, Meli+ extra installments, R$ 5 minimum installment.
- **[ML-24b]** https://www.mercadolivre.com.br/ajuda/3143 — "Como libero o dinheiro": payout table (8/12 days), 11/28 days MercadoLíder vs 5/28 others.
- **[ML-25]** https://www.mercadolivre.com.br/ajuda/Compra-garantida_601 — Compra Garantida terms (last modified 29/09/2025): deadlines 30/12/28/60 days, seller bears return cost, exclusions (vehicles, real estate), sanctions.
- **[ML-26]** https://www.mercadolivre.com.br/ajuda/devolucoes-expressas_3448 — returns from seller side: 30 days new / 5 days used, cost and reputation impact by reason, Full return handling.
- **[ML-27]** https://www.mercadopago.com.br/ajuda/programa-protecao-vendedor_527 — Programa de Proteção ao Vendedor (updated 09/09/2025): chargeback coverage requirements and exclusions.
- **[ML-28]** https://vendedores.mercadolivre.com.br/nota/como-os-envios-do-mercado-livre-funcionam — Envios modes (Full, Flex, Coleta, Agências), labels, tracking, free-shipping rules (R$ 19-78,99 paid by ML; >= R$ 79 seller pays with up to 70% off), "48 horas" payout claim.
- **[ML-30]** https://www.mercadolivre.com.br/ajuda/Como-calcular-o-custo-dos-seus_4413 — billable weight (volumetric /6000), re-measurement, cost per sale.
- **[ML-31]** https://www.mercadolivre.com.br/ajuda/40538 — Envios cost table for MercadoLíder/green/no-reputation sellers by weight and price band; up to 50% green discount.
- **[ML-32]** https://vendedores.mercadolivre.com.br/nota/frete-gratis-a-partir-de-r-19-o-que-muda-pro-vendedor — permanent free shipping from R$ 19, Flat Fee rule, 40%/55% cost reductions.
- **[ML-33]** https://www.mercadolivre.com.br/ajuda/Termos-e-condicoes-compra-internacional_3300 — international purchase terms (import costs, price composition, returns).
- **[ML-34]** https://developers.mercadolivre.com.br/pt_br/itens-e-buscas — search API: available_filters, available_sorts (relevance default, price_asc/desc), healthy/unhealthy item flags.
- **[ML-35]** https://www.mercadolivre.com.br/ajuda/an-ncios-de-cat-logo_4904 — Catalog terms (updated 21/08/2025): winner criteria, "Outras opções de compra", Recomendado badge.
- **[ML-36]** https://vendedores.mercadolivre.com.br/nota/como-a-concorrencia-funciona-no-catalogo — catalog competition statuses, buyer-dependent featured seller.
- **[ML-37]** https://www.mercadolivre.com.br/ajuda/anuncios-categoria-correta_1022 — wrong-category handling (auto-change or cancellation).
- **[ML-39]** https://www.mercadolivre.com.br/ajuda/Termos-e-condicoes-gerais-de-uso_1500 — Envios terms (updated 14/10/2024): label generation with NF-e key, Coleta/Agências, lost-parcel compensation, Correios/carriers, penalties.
- **[ML-41]** https://envios.mercadolivre.com.br/envios-extra/ajuda/3228 — buyer view: cost/ETA in listing, tracking, free shipping from R$ 79, arrange delivery with seller if no Envios.
- **[ML-42]** https://www.mercadolivre.com.br/ajuda/Sistema-de-opiniones-de-produc_1911 — product review system (last updated 21/set/2015): purchase required, 1-5 stars, moderation.
- **[ML-43]** https://www.mercadolivre.com.br/ajuda/26454 — review policies: photos/videos, rewards points/coupons, one review per product, AI use.
- **[ML-44]** https://www.mercadolivre.com.br/ajuda/21062 — reputation color thresholds per variable.
- **[ML-45]** https://www.mercadolivre.com.br/ajuda/como-se-tornar-mercadolider_1359 — MercadoLíder / Gold / Platinum requirements.
- **[ML-46]** https://www.mercadolivre.com.br/ajuda/Quando-e-como-qualificar-o-vendedor_928 — buyer rates seller after delivery; vehicles/real estate/services outside reputation.
- **[ML-47]** https://developers.mercadolivre.com.br/pt_br/feedback-de-uma-venda — order feedback API (updated 29/12/2025): seller rates counterpart, no reputation impact.
- **[ML-48]** https://developers.mercadolivre.com.br/pt_br/gerenciamento-perguntas-respostas — Q&A before purchase, BANNED status.
- **[ML-49]** https://www.mercadolivre.com.br/ajuda/perguntas-em-um-anuncio_930 — sellers can block buyers from asking.
- **[ML-51]** https://developers.mercadolivre.com.br/pt_br/com-pausa — preventive pauses (price change, abandoned items, image URL), reactivation.
- **[ML-52]** https://www.mercadolivre.com.br/ajuda/Produtos-proibidos_1029 — prohibited products policy and sanctions.
- **[ML-54]** https://www.mercadolivre.com.br/ajuda/25193 — suspension causes, temporary vs permanent effects, how to resolve.
- **[ML-55]** https://www.mercadolivre.com.br/ajuda/dicas_veiculos_3913 — vehicle buyer safety tips, FIPE guide link.
- **[ML-56]** https://developers.mercadolivre.com.br/pt_br/publicacao-de-automoveis — vehicles are classifieds with public contact info, no transaction; LICENSE_PLATE attributes; MLB item in BRL; phone2 mandatory for dealers from 01/10/2026.
- **[ML-57]** https://vendedores.mercadolivre.com.br/nota/contate-os-compradores-com-credito-pre-aprovado-para-a-venda-de-carros-no-mercado-livre — "Simular Financiamento", pre-approved credit leads, WhatsApp contact.
- **[ML-58]** https://vendedores.mercadolivre.com.br/nota/reserva-online-de-veiculos-melhore-suas-vendas-com-esse-novo-recurso/ — Reserva Online flow via Mercado Pago, reputation visible from 5th completed sale.
- **[ML-59]** https://www.mercadolivre.com.br/ajuda/16467 — "Como funcionam os fretes grátis": thresholds R$ 19 / R$ 79, cart aggregation.

### OLX Brasil (OLX)

- **[OLX-1]** https://ajuda.olx.com.br/ — help center home; operator legal name/address; lang pt-BR.
- **[OLX-2]** https://ajuda.olx.com.br/s/article/olx-e-gratis — "A OLX é grátis": zero sale fee, R$ 29,90 / +5% benefits, renewal rules, wallet fees, guarantee fee structure.
- **[OLX-3]** https://ajuda.olx.com.br/s/article/tarifas-compra-venda-online — guarantee tariffs; dynamic R$ 25,90–27,90 free-shipping fee; +5%.
- **[OLX-4]** https://ajuda.olx.com.br/s/article/lista-de-produtos-garantia-olx — categories with payment+delivery, payment-only, and excluded (Autos, Imóveis, pets, services, jobs).
- **[OLX-5]** https://ajuda.olx.com.br/s/article/termos-e-condicoes-fretegratis — Free shipping T&C: R$ 29,90 fee, ≥ R$ 50 items, Correios only, ≤ R$ 30 quote.
- **[OLX-6]** https://ajuda.olx.com.br/s/article/termos-e-condicoes-frete-gratis-parcelamento — Free shipping + 10x interest-free T&C: R$ 29,90 + 5%, rounding, min installment R$ 30.
- **[OLX-7]** https://ajuda.olx.com.br/s/article/anuncio-pago-e-limites-de-insercao-gratuita — free listing limits per subcategory; Autos 1/6 months, Imóveis 1/3 months.
- **[OLX-8]** https://vender.olx.com.br/ — seller landing: "1 a 40 anúncios grátis por mês", "Planos a partir de R$ 59,90", buyer pays shipping and guarantee fee.
- **[OLX-10]** https://ajuda.olx.com.br/s/article/contratar-anuncio-extra — extra ad purchase, "Acima do limite", non-refundable.
- **[OLX-11]** https://ajuda.olx.com.br/s/article/como-funcionam-planos-profissionais — plan mechanics, billing periods, features, lead collector, prices vary by location.
- **[OLX-13]** https://ajuda.olx.com.br/s/article/conheca-os-planos-profissionais — Veículos plans Essencial/Plus/Essencial Empresa/Plus Empresa/Premium Empresa and features.
- **[OLX-17]** https://ajuda.olx.com.br/s/article/funcionamento_destaque — highlight types (Básico, Prata, Ouro, Diamante, Ouro Quinzenal, Diamante Mensal), durations, no refund.
- **[OLX-18]** https://ajuda.olx.com.br/s/article/destaques-da-olx — how highlights work, "Impulsionado" top-4, sponsored gallery, PIX min R$ 10.
- **[OLX-22]** https://ajuda.olx.com.br/s/article/quero-criar-uma-conta-na-olx — sign-up fields (PF/PJ, CPF/CNPJ).
- **[OLX-23]** https://ajuda.olx.com.br/s/article/regras — Rules: private vs professional ads, one account, one phone/e-mail, duplicates, content in Portuguese, items must be in Brazil.
- **[OLX-25]** https://ajuda.olx.com.br/s/article/como-e-por-que-verificar-o-seu-perfil-na-OLX — biometric identity verification: RG/CNH, 24 h, badge, CPF lock, 30-day transfer block.
- **[OLX-28]** https://ajuda.olx.com.br/s/article/como-cadastrar-carteira — wallet requirements (CPF regular, 18+, phone/e-mail), 48 business hours activation.
- **[OLX-29]** https://ajuda.olx.com.br/s/article/termos-e-condicoes-garantia-da-olx-e-entrega-facil — Garantia/Entrega Fácil T&C (updated 26 Aug 2026): payment methods, 5-day shipping, 48 h release, mediation, refunds, covered situations, termination notices.
- **[OLX-30]** https://adevinta.com/press-releases/adevinta-and-prosus-jv-olx-brasil-appoints-new-chief-executive-officer-olivier-aizac-takes-over-from-andries-oudshoorn/ — Prosus 50% / Adevinta 50% JV (search snippet only, secondary).
- **[OLX-33]** https://portogente.com.br/noticias/transporte-logistica/103357-olx-lanca-vertical-especializada-em-carros — Portogente, 2018-08-15 (secondary, press release): Autoshift launch, dealer-only, no extra charge, ≥ R$ 10k cars, filters.
- **[OLX-35]** https://www.nuvemshop.com.br/blog/como-vender-na-olx/ — Nuvemshop blog, 2025-09-18 (secondary): highlight prices R$ 20,99 / 25,99 / 33,99.
- **[OLX-36]** https://ajuda.olx.com.br/s/article/termos-e-condicoes-gerais-de-uso-carteira-olx — Carteira OLX T&C: KYC checks (Receita, SPC/Serasa), settlement 10 business days/30 days, security block 10 days, contestation, inactivity fee, 2% fine + 1%/month.
- **[OLX-37]** https://ajuda.olx.com.br/s/article/termos-e-condicoes-agro-e-industria — Agro guarantee terms: R$ 10 to R$ 20.000 eligibility.
- **[OLX-38]** https://ajuda.olx.com.br/s/article/como-pagar-compras — buyer payment methods: PIX (5 min), cards/brands, NuPay (10 min), installments from R$ 30, card up to 2 business days.
- **[OLX-39]** https://ajuda.olx.com.br/s/article/entrega-e-pagamento-online — how to buy: offers (min R$ 10, 48 h), delivery options incl. Clique Retire locker 72 h, escrow flow.
- **[OLX-40]** https://ajuda.olx.com.br/s/article/anunciar-entrega-e-pagamento — how to sell: R$ 10–20.000, paid after delivery confirmation, transfer up to 2 business days, seller paid upfront on installments, weight not editable.
- **[OLX-41]** https://ajuda.olx.com.br/s/article/sacar-dinheiro-da-conta — wallet balance/withdrawal, min R$ 1,00, free, automatic.
- **[OLX-42]** https://ajuda.olx.com.br/s/article/como-funciona-a-mediacao — agreement vs mediation: 48 h window, 5-day return, evidence by e-mail, not insurance.
- **[OLX-44]** https://ajuda.olx.com.br/s/article/entrega-com-postagem — drop-off shipping: label in 24 h, DACE, carriers and size/weight limits (Correios, Loggi, Jadlog, J&T).
- **[OLX-45]** https://ajuda.olx.com.br/s/article/entrega-com-coleta — home pickup: 99 Entregas Flash, Loggi, R3 limits; 1 h courier acceptance.
- **[OLX-46]** https://ajuda.olx.com.br/s/article/entrega-flash — Entrega Flash (99), delivery within 24 h.
- **[OLX-48]** https://ajuda.olx.com.br/s/article/como-fazer-busca — search: sorting options, filters (category, price, guarantee benefits, advertiser type), location.
- **[OLX-50]** https://dicas.olx.com.br/autos/melhorar-seus-anuncios-na-olx/ — OLX blog: attribute fields act as filters (car year/model, rooms).
- **[OLX-52]** https://ajuda.olx.com.br/s/article/regras — (same as 23) rules 4, 7, 8, 12: duplicates, Portuguese, categories, Brazil-only.
- **[OLX-53]** https://ajuda.olx.com.br/s/article/anuncio-removido-ou-nao-esta-ativo — refusal reasons: unrealistic price, wrong category, duplicates, automation.
- **[OLX-54]** https://ajuda.olx.com.br/s/article/avaliar-minha-compra — buyer rating flow: 1–5 stars, 3 reasons, carrier, NPS, 15 days "em análise".
- **[OLX-55]** https://ajuda.olx.com.br/s/article/avaliar-minha-venda — seller rates buyer, visibility rules.
- **[OLX-56]** https://ajuda.olx.com.br/s/article/meu-perfil — profile and reputation levels Básico/Intermediário/Avançado/Especialista.
- **[OLX-57]** https://ajuda.olx.com.br/s/article/anuncio-com-selo-vistoriado — Vistoriado badge: Detran-accredited partners, 120 days, free badge.
- **[OLX-59]** https://ajuda.olx.com.br/s/article/telefone-no-anuncio — phone hidden on private ads; pro ads may show; lead sharing.
- **[OLX-60]** https://ajuda.olx.com.br/s/article/como-usar-chat — chat features, 90-day history, block/report.
- **[OLX-61]** https://ajuda.olx.com.br/s/article/como-publicar-anuncio — publishing: photo limits (6 / 20), plate mandatory, 3-day edit window, 24 h activation, statuses, title/description limits.
- **[OLX-62]** https://ajuda.olx.com.br/s/article/produtos-e-servicos-proibidos-na-olx — prohibited ads list; account block warning.
- **[OLX-64]** https://ajuda.olx.com.br/s/article/termos-e-condicoes-de-uso — T&C landing (updated 27 Feb 2025) linking to PDF [24].
- **[OLX-66]** https://ajuda.olx.com.br/s/article/denunciar-anuncio — anonymous reporting, 3 photos, ~24 h handling.
- **[OLX-69]** https://ajuda.olx.com.br/s/article/historico-veicular — Histórico Veicular report contents, purchase flow (CPF/CNPJ, plate, RENAVAM), 72 h activation.
- **[OLX-71]** https://ajuda.olx.com.br/s/article/financiamento-veicular — financing: Santander (PF, ≤ 20 years), Safra (PJ, ≤ 15 years), flow.
- **[OLX-74]** https://ajuda.olx.com.br/s/article/dicas-compra-venda-veiculos — vehicle buying/selling safety tips, DETRAN transfer after payment.

### Shopee Brasil (SHP)

- **[SHP-1]** https://help.shopee.com.br/portal/4/article/76155 — "O que é Shopee": platform description, categories (profile, no vehicle category).
- **[SHP-3]** https://help.shopee.com.br/portal/article/77113 — Terms of Service (updated 26 June 2026): operating entity, Garantia Shopee escrow (12.1–12.2), Taxa de Transação, international sellers (11.x), suspension reasons (5.x), age.
- **[SHP-5]** https://ecommercenapratica.com/blog/taxa-shopee/ — secondary: 2026 fee table by price band, CPF +R$3, cap removal, mandatory free shipping.
- **[SHP-6]** https://blog.calcularte.com.br/index.php/2026/03/06/taxas-comissoes-shopee-2026/ — secondary: 50% for items < R$8, subsidy caps R$20/30/40, Pix subsidy 5–8%.
- **[SHP-7]** https://abaccus.com.br/blog-posts/shopee-mudou-as-regras-de-comissao-2026 — secondary: announcement 4 Feb 2026, effective 1 March 2026, cites edu/article/26839.
- **[SHP-9]** https://tributei.net/blog/como-precificar-na-shopee/ — secondary: 14% + 6% freight program, still cites R$100 cap (conflict).
- **[SHP-10]** https://seller.shopee.com.br/edu/article/26839/Comissao-para-vendedores-CNPJ-e-CPF-em-2026 — official fee article (exists, JS-rendered, content not retrievable).
- **[SHP-11]** https://ads.shopee.com.br/learn/faq/217/906 — Shopee Ads billing: CPC, R$15 minimum, payment methods, no refunds, invalid clicks.
- **[SHP-12]** https://ads.shopee.com.br/learn/faq/111/1464 — definition of bid and CPC.
- **[SHP-14]** https://ads.shopee.com.br/learn/faq/473/1994 — Custo por Pedido (CPP) rules within GMV Max.
- **[SHP-15]** https://ads.shopee.com.br/learn/faq/339/1496 — search snippet only: R$50 minimum recharge.
- **[SHP-17]** https://deo.shopeemobile.com/shopee/seller/seller_cms/770c985c69a5a2741a1c6aec60cc5c8b/CPF%20para%20CNPJ.pdf — official webinar deck: CPF→CNPJ migration steps, KYC, timelines, Ads formats.
- **[SHP-18]** https://blog.arcosscale.com.br/checklist-documentos-necessarios-vender-shopee/ — secondary: registration documents CPF/CNPJ, R$81k threshold.
- **[SHP-20]** https://www.lexos.com.br/blog/como-recebo-meu-pagamento-no-shopee/ — secondary: bank account ownership rules, withdrawal up to 5 business days.
- **[SHP-21]** https://help.shopee.com.br/portal/4/article/76237 — buyer payment methods (card, Google Pay, Pix, SParcelado, Maree, boleto, Caixa virtual debit).
- **[SHP-22]** https://shopee.com.br/blog/metodos-de-pagamento/ — official blog: payment methods, two-card payment.
- **[SHP-24]** https://help.shopee.com.br/portal/4/article/76367 — installments up to 12x, R$5 minimum per installment.
- **[SHP-25]** https://help.shopee.com.br/portal/4/article/76281 — no fee for one-off card payment, R$60 minimum for no-surcharge installment, IOF on international.
- **[SHP-27]** https://help.shopee.com.br/portal/article/76318 — Termos e Condições de Serviços de Pagamento Para Vendedores (PDF): 7-day Garantia Shopee hold, chargeback offsets, tariffs clause.
- **[SHP-29]** https://shopee.com.br/blog/o-que-e-a-garantia-shopee/ — official blog: 3+7+7 = 17-day example, auto release.
- **[SHP-30]** https://gosmarter.com.br/repasse-shopee-ciclo-pagamento/ — secondary: 3 free withdrawals/week, R$5 extra, 3–5 business days.
- **[SHP-31]** https://help.shopee.com.br/portal/4/article/77827 — Return and Refund Policy: 30/90-day windows, who pays return shipping, 3-day inspection, 30-day contest.
- **[SHP-33]** https://help.shopee.com.br/portal/4/article/76307 — refund timing by payment method.
- **[SHP-34]** https://blog.destraveescale.com.br/nova-taxa-de-devolucao-e-reembolso-entrou-em-vigor-na-shopee/ — secondary: seller return fee R$10 (from R$25) since 12 July 2023.
- **[SHP-35]** https://help.shopee.com.br/portal/4/article/76332 — shipping fee calculation, per-seller shipping.
- **[SHP-36]** https://help.shopee.com.br/portal/article/124091 — Logistics Program Terms: freight criteria, mandatory use of enabled logistics.
- **[SHP-37]** https://help.shopee.com.br/portal/4/article/76220 — free-shipping coupon rules (R$20 cap, 10/week, 3 packages).
- **[SHP-38]** https://help.shopee.com.br/portal/4/article/76265 — Programa de Frete Grátis (buyer view).
- **[SHP-39]** https://help.shopee.com.br/portal/4/article/76163 — Shopee Xpress (SPX) pickup for selected sellers.
- **[SHP-40]** https://help.shopee.com.br/portal/4/article/164642 — Agência Shopee pickup with PIN.
- **[SHP-41]** https://help.shopee.com.br/portal/4/article/76363 — cross-border parcels delivered by Correios.
- **[SHP-42]** https://help.shopee.com.br/portal/4/article/76322 — tracking of international orders.
- **[SHP-43]** https://blog.bling.com.br/shopee-envios/ — secondary: Shopee Envios labels, seller prints, Correios drop-off/pickup.
- **[SHP-46]** https://help.shopee.com.br/portal/4/article/76162 — search sorting options.
- **[SHP-47]** https://help.shopee.com.br/portal/4/article/76198 — Vendedores Indicados badge and filter.
- **[SHP-48]** https://blog.yescommerce.com.br/glossario/como-fazer-navegacao-por-filtros-avancados-no-shopee/ — secondary: filter types.
- **[SHP-49]** https://help.shopee.com.br/portal/4/article/81649 — "Similar" option in cart (no compare tool).
- **[SHP-50]** https://help.shopee.com.br/portal/4/article/76377 — review rules: 30 days, 5 stars, coins, 3 sub-criteria.
- **[SHP-51]** https://help.shopee.com.br/portal/4/article/77973 — review editable once within 30 days.
- **[SHP-52]** https://help.shopee.com.br/portal/4/article/76444 — review removal reasons; seller replies moderated.
- **[SHP-53]** https://help.shopee.com.br/portal/4/article/78029 — Lojas Oficiais (Shopee Mall) benefits.
- **[SHP-54]** https://ecommercenapratica.com/blog/vendedor-indicado-shopee/ — secondary: Vendedor Indicado thresholds, Antecipa 3.5%→2%.
- **[SHP-56]** https://help.shopee.com.br/portal/4/article/78191 — private chat with sellers.
- **[SHP-57]** https://help.shopee.com.br/portal/4/article/78195 — price offers via chat.
- **[SHP-58]** https://help.shopee.com.br/portal/4/article/76225 — Listing Violation Guide: violation types, penalties, 28-day quota limit, off-platform contact ban.
- **[SHP-59]** https://help.shopee.com.br/portal/4/article/77326 — policy index (links to prohibited-items policy at edu/article/3304, Frete Grátis Extra terms).
- **[SHP-60]** https://help.shopee.com.br/portal/4/article/76226 — Prohibited and Restricted Products policy (search snippet only; page not retrievable).
- **[SHP-61]** https://help.shopee.com.br/portal/10/article/188352 — "Automóveis" prohibited/restricted list (Shopee Video/Live portal).
- **[SHP-62]** https://deo.shopeemobile.com/shopee/seller/seller_cms/6be0c3464c0b8a45f8269e7bf3461ce4/Pontos%20de%20Penalidade.pdf — official penalty-points deck (2021): pillars, quarterly, 28-day loss of benefits.
- **[SHP-63]** https://gobots.ai/blog/pontos-de-penalidade-shopee/ — secondary, search snippet only: 3-point and 6-point tiers.
- **[SHP-65]** https://help.shopee.com.br/portal/4/article/76251 — account limitation reasons, appeal documents, 30-day/5-day deadlines.
- **[SHP-67]** https://help.shopee.com.br/portal/4/article/76263 — app languages: Portuguese and English.
- **[SHP-68]** https://help.shopee.com.br/portal/4/article/78028 — "produtos do exterior" definition and tag.
- **[SHP-69]** https://help.shopee.com.br/portal/4/article/134143 — Remessa Conforme: 20% / 60% minus USD 20, ICMS 17–20%, since 27 July 2024.

### Amazon Brasil (AMZ)

- **[AMZ-1]** https://venda.amazon.com.br/precos — plans (R$ 2/item; R$ 19/month from month 13), commission table updated 20/01/2025 with min R$ 1/R$ 2, 1,5% installment program, FBA fee tables (01/08/2025), DBA fees, free-shipping tiers, R$ 250 ads credit.
- **[AMZ-3]** https://venda.amazon.com.br/venda — thresholds ODR <1%, late shipment <4%, pre-shipment cancellation <2.5%; restricted categories; single detail page per product; "70% ... primeira venda em menos de 60 dias".
- **[AMZ-4]** https://venda.amazon.com.br/ajuda — FAQ: who can sell (CPF/CNPJ, bank account, card), R$ 1 verification charge, biweekly payouts, up to 10x sem juros, Individual plan shipping set by Amazon, international cards accepted.
- **[AMZ-5]** https://venda.amazon.com.br/guia-cadastro-vendedor-amazon — registration steps, documents, selfie, video call, 2 business days verification, country selector note.
- **[AMZ-6]** https://venda.amazon.com.br/guia-verificacao-documentacao — accepted documents for CPF/CNPJ/MEI, 180-day proof of address, foreign-company section, limited access until verified.
- **[AMZ-8]** https://venda.amazon.com.br/cresca/dba — DBA description, carriers Total Express/Correios, fee tiers R$ 4,50/6,50/6,75, SP 50%-off campaign, automatic returns with prepaid labels.
- **[AMZ-9]** https://venda.amazon.com.br/cresca/dba/guia-do-vendedor — DBA open to CPF and CNPJ, default for new sellers, eligibility (22 kg, R$ 20.000), pickup Mon-Fri, AMZL, Inscrição Estadual rule.
- **[AMZ-10]** https://venda.amazon.com.br/cresca/dba/central-de-treinamento/performance-operacional — DBA metrics: late shipment <2,5% (10/30 days), cancellation <2,5% (7 days), carrier exemptions.
- **[AMZ-11]** https://venda.amazon.com.br/termos/frete19 — free-shipping terms by program (R$ 19 / R$ 79 tiers, regional rule for DBA).
- **[AMZ-12]** https://venda.amazon.com.br/pontos-de-envio — DBA drop-off points, labels required, scanning.
- **[AMZ-13]** https://venda.amazon.com.br/publicidade — CPC model, daily budget, Professional account requirement, Buy Box eligibility for Sponsored Products, Brand Registry for Sponsored Brands, moderation of SB/Stores.
- **[AMZ-15]** https://advertising.amazon.com/pt-br/solutions/products/sponsored-products — max bid per click, daily budget, eligibility; no minimum bid stated.
- **[AMZ-16]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GFBWMNXEPYVJAY9A — accepted payment methods (Visa, Mastercard, Elo, Amex, Pix, NuPay, Livelo, Geru 3-24x, gift card), combination rules.
- **[AMZ-16b]** https://venda.amazon.com.br/sellerblog/o-que-fazer-para-ganhar-a-oferta-em-destaque-amazon-confira-6-dicas and https://venda.amazon.com.br/sellerblog/conta-de-vendedor-suspensa-na-amazon — Buy Box criteria; suspension process, plano de ação, metric windows (60/30/7 days, tracking >95%, on-time >97%). Official Amazon seller blog.
- **[AMZ-17]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GMDABF4BPHS5PNQ8 — Pix: 30-minute window, 10 minutes for Pix Automático, valid for Amazon and 3P sellers.
- **[AMZ-19]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GQ37ZCNECJKTFYQV — A-to-Z guarantee conditions (90 days, 48 h, 3 days, 7 days), exclusions, chargeback exclusion.
- **[AMZ-19b]** https://venda.amazon.com.br/vender-marketplace/venda-automotivos — automotive category guide (parts, accessories, tyres; approval; no vehicle-maker brands).
- **[AMZ-20]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GSZAYH7K2C2NVNC9 — A-to-Z request; "até uma semana" for decision.
- **[AMZ-21]** https://sellercentral.amazon.com.br/seller-forums/discussions/t/05fe29a2-2b7e-4f1a-b33e-0c2c15a9b5a6 — Amazon staff post (Angie_Amazon): payments every two weeks, DD+7 reserve, account-level reserves. Secondary (forum) source, Amazon-authored.
- **[AMZ-22]** https://sellercentral.amazon.com.br/seller-forums/discussions/t/ed3efbf6-2c5f-4106-863e-db75969a303f — Amazon staff post: up to 5 business days for deposit to appear, 3-day reserve after bank changes, DD+7. Secondary (forum) source, Amazon-authored.
- **[AMZ-24]** https://www.amazon.com.br/dp/B09G9FPHY6 — product page: "Nenhuma oferta em destaque disponível", "Outros vendedores", comparison widget IDs, "Perguntas" section, lang pt-br, footer payment methods and country chooser, company CNPJ.
- **[AMZ-25]** https://www.amazon.com.br/s?k=smart+tv+50+polegadas&i=electronics — sort options and attribute filters.
- **[AMZ-26]** https://www.amazon.com.br/b?ie=UTF8&node=18914209011 — Automotivo store subcategories (parts/accessories only, no vehicles).
- **[AMZ-27]** https://www.aboutamazon.com.br/noticias/loja/amazon-lanca-cartoes-de-credito-veja-opcoes-de-pagamento-aceitas-no-site — boleto validity 1 business day/3-day approval, 12x with R$ 50 minimum instalment, Amazon card 15x.
- **[AMZ-29]** https://venda.amazon.com.br/sellerblog/o-que-voce-mais-precisa-saber-sobre-chargeback-no-e-commerce — chargeback borne by the store; 7-day right of withdrawal.
- **[AMZ-30]** https://www.aboutamazon.com.br/noticias/noticias-da-empresa/como-funcionam-os-pontos-de-retirada-amazon — 532 pickup points, Jadlog partnership, 3.000 target, 150 million products, free shipping above R$ 79 for Amazon-shipped items.
- **[AMZ-31]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GZTXM2L3YLZFSQ6W — pickup point rules, 10 calendar days hold.
- **[AMZ-32]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=G8UYX7LALQC8V9KA — how reviews work, AI-based star rating, verified-purchase weighting, R$ 50 rule.
- **[AMZ-33]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=G75XTB7MBMBTXP6W — "Compra verificada" definition.
- **[AMZ-34]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GLHXEX85MENUE4XF — Community Guidelines: R$ 50 in 12 months, photos/videos, Q&A rules, incentivised-review ban, reporting.
- **[AMZ-36]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=G5T39MTBJSEVYQWW — seller feedback: 90 days, 1-5 stars, removal rules.
- **[AMZ-37]** https://www.amazon.com.br/customer-preferences/edit?ie=UTF8&preferencesReturnUrl=%2F and https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GARKQZZYZ542RGWK — no language selector; language-preference help page not found on .com.br.
- **[AMZ-38]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=G26L6NHEDGERVR8W — customs and taxes for international purchases (US$ 50 threshold, 60% import tax, ICMS ~17%, IBS/CBS 2026), products from China and USA.
- **[AMZ-39]** https://www.amazon.com.br/gp/help/customer/display.html?nodeId=GJF6884LHHZ5ELD4 and https://www.amazon.com.br/gp/help/customer/display.html?nodeId=Tusv6Jq8x8VfmZuQ7z — "Compras internacionais" hub and international terms (returns, 2-business-day seller response).
- **[AMZ-40]** https://venda.amazon.com.br/cresca/venda-global — selling in the US from Brazil, incentives, unified account fee note.
- **[AMZ-43]** Not retrievable (login-only JS shells, cited for completeness): https://sellercentral.amazon.com.br/help/hub/reference/external/G200285170 (ODR), G200285190 (late shipment), G200285210 (cancellation), GGJVNFDXQT8C3RA8 (order performance policy), G14911 (payout timing), G201112670 (FBA fees), 201382050 (DBA fees), 200164330 (restricted products) — content `not found`.

### Magalu (MGL)

- **[MGL-1]** https://universo.magalu.com/ — official; ecosystem, CNPJ >3 months + NF-e, "comissão + custo fixo por pedido", no monthly fee, 9.9% promo.
- **[MGL-2]** https://universo.magalu.com/blog/artigo/vendaagora — official (updated 14 Sep 2026); "De 18% por apenas 9,9%", 3 months / R$ 100k, benefits until first quality indicator.
- **[MGL-3]** https://universo.magalu.com/blog/artigo/programaaceleracaosellers — official; PAS 13%/11%, 6 months, R$ 300k, 40/80 orders, reputation >3.0, ad credits.
- **[MGL-4]** https://universo.magalu.com/blog/artigo/regra-repasse — official (updated 14 Sep 2026); payout table (Despacho+3 / Entrega+7 / Despacho+28), MagaluPay, R$ 5 transfer fee waiver.
- **[MGL-5]** https://ps-core-universo-magalu-storage.magazineluiza.com.br/media/tabela_repasse_717309d743.png — official image with the payout-days table.
- **[MGL-6]** https://universo.magalu.com/blog/artigo/acordo-de-nivel — official SLA (updated 16 Sep 2026); 95%, 98%, <5%, <1%, day counts.
- **[MGL-7]** https://universo.magalu.com/blog/artigo/comercia-proibida — official prohibited products and penalties.
- **[MGL-8]** https://universo.magalu.com/blog/artigo/inegociaveis-magalu-marketplace — official (updated 14 Sep 2026); grave faults → suspension.
- **[MGL-9]** https://universo.magalu.com/blog/artigo/pacto-de-integridade — official (updated 13 Sep 2026); integrity pact, immediate termination.
- **[MGL-10]** https://magaluads.com.br/termos-de-uso/ — official Magalu Ads terms; CPC/CPM, R$ 50 minimum, category minimum CPCs, anti-fraud, billing.
- **[MGL-11]** https://magaluads.zendesk.com/hc/pt-br/articles/26684626685975-Quanto-eu-vou-pagar-H%C3%A1-alguma-taxa — official; CPC, R$ 50 top-up, 15% offsite fee, no expiry.
- **[MGL-13]** https://www.magaluentregas.com.br/ — official; shared shipping cost, >97% dispatch → up to 50% discount above R$ 79, modalities, labels, carriers.
- **[MGL-14]** https://www.magaluentregas.com.br/nossa-rede/agencia-magalu-entregas — official; 30 kg / 100 cm / 200 cm limits, R$ 0.50 per package, hours.
- **[MGL-16]** https://assets.mlcdn.com.br/conteudo/regulamentos/regulamentomagalu1.html — official; buyer free shipping R$ 99 / R$ 699, not North, not partners.
- **[MGL-17]** https://especiais.magazineluiza.com.br/termo-compra-venda/ — official buyer terms; CNPJ/HQ, partner-store responsibility, 7-day withdrawal/exchange, 9 items/5 units, card brands.
- **[MGL-18]** https://atendimento.magazineluiza.com.br/hc/pt-br/articles/360046317951-Quais-s%C3%A3o-as-formas-de-pagamento-dispon%C3%ADveis — official; payment methods, two cards, Carnê Digital.
- **[MGL-19]** https://atendimento.magazineluiza.com.br/hc/pt-br/articles/15038627327757-Quais-s%C3%A3o-as-formas-de-pagamento-disponibilizadas — official; Pix key 5 min, "conforme a política de cada mercado".
- **[MGL-20]** https://atendimento.magazineluiza.com.br/hc/pt-br/sections/32266943176973-Envio-Internacional — official; international shipping topics.
- **[MGL-21]** https://atendimento.magazineluiza.com.br/hc/pt-br/articles/16138878094477-Qual-o-prazo-de-entrega-do-meu-pedido-internacional — official; 6 business days dispatch.
- **[MGL-22]** https://assets.mlcdn.com.br/conteudo/regulamentos/termos_e_condicoes_review.html — official review terms; approval, 2–4 business days.
- **[MGL-23]** https://luizalabs.substack.com/p/a-importancia-do-buybox-no-magazine — official Luizalabs (2026-02-19); buy box criteria, "outras ofertas".
- **[MGL-24]** https://www.parceiromagalu.com.br/divulgador/tudo-sobre-comissionamento.html — official affiliate program; up to 12%, levels, R$ 50 minimum, fortnightly payment.
- **[MGL-25]** https://consorciomagalu.com.br/ver-veiculos/ — official consortium page; vehicle consortium, not listings.
- **[MGL-28]** https://blog.arcosscale.com.br/comissao-magazine-luiza-taxas-vender-marketplace/ — secondary; category commission table 16–19.9%.
- **[MGL-29]** https://basedoecommerce.com.br/guia-como-vender-no-magalu/ — secondary; same table, KYC documents, MEI R$ 81k.
- **[MGL-34]** https://www.cnnbrasil.com.br/economia/negocios/magazine-luiza-lanca-parcelamento-em-21-vezes-sem-juros/ — secondary (2024-08-29); 21x own card, 10x other cards.
- **[MGL-35]** https://www.infomoney.com.br/consumo/magazine-luiza-e-aliexpress-iniciam-parceria-para-vendas-cruzadas/ ; https://www.cnnbrasil.com.br/economia/negocios/magazine-luiza-e-aliexpress-fecham-acordo-para-vender-produtos-em-ambos-os-marketplaces/ — secondary; AliExpress partnership (2024-10-13), Remessa Conforme.
- **[MGL-36]** https://lojahub.com.br/blog/taxa-comissao-marketplaces-comparativo/ ; https://base.com/pt-BR/blog/como-vender-na-magalu-marketplace/ — secondary, search snippet only; fixed fee R$ 3–5 above R$ 10, 48 h approval, catalog linking.
- **[MGL-37]** https://cupomlu.com.br/innovation-magazine-luiza-filtrar-produtos-vendidos-diretamente/ ; https://www.reclameaqui.com.br/magazine-luiza-loja-online/falta-um-filtro-para-produtos-vendidos-e-entregues-pela-magalu_lD0egXtfDCmET09g/ — secondary, search snippet only; filter and sort labels.
- **[MGL-38]** https://m.magazineluiza.com.br/q/a/ed05h18jc7/undefined/CI/CMRI/ — official page (403 on fetch), search snippet only; Q&A with "Comprou!".
- **[MGL-39]** https://www.reclameaqui.com.br/magazine-luiza-loja-online/avaliar-compra_xEECr7yj7oK7umGi/ — secondary, search snippet only; reviews from "Seus pedidos", product or partner store.

### KaBuM! (KBM)

- **[KBM-1]** https://www.kabum.com.br/hotsite/confiavel/ — profile, Magalu ownership since 2021, scale figures, certifications.
- **[KBM-2]** https://www.kabum.com.br/hotsite/marketplace/ — official seller hotsite: 18% commission, seller rules (CNPJ 3 months, NF-e, no dropshipping), registration steps (1 dia útil), repasse cycle (5º dia útil), BuyBox, Seller Score, allowed categories, traffic claims.
- **[KBM-3]** https://www.kabum.com.br/politicas — site policies: payment methods, PIX 20 min, installments vary by product, Loja Parceira clauses, 7-day withdrawal, OpenBox 90-day legal warranty, Q&A disclaimer, fraud cancellation, credits 12 months.
- **[KBM-4]** https://www.kabum.com.br/hotsite/pix/ — PIX "até 15% OFF", 30-minute payment window.
- **[KBM-6]** https://blog.kabum.com.br/como-vender-no-kabum-marketplace/ — official blog (2025-07-11): 18% commission, contact in 5 dias úteis, cycle 01–31 paid "dia 7", 60M page views.
- **[KBM-8]** https://www.kabum.com.br/hardware/placa-de-video-vga — category page: sort options, filters, no "Comparar".
- **[KBM-9]** https://www.kabum.com.br/produto/989622/placa-de-video-palit-geforce-rtx-5070-white-oc-12gb-gddr7-192bit-3-dp-e-hdmi-ne75070u19k9-gb2050w — product page: PIX price vs card price, "10x s/ juros", "36x no NuPay", warranty text, rating JSON, lang=pt-br, "Vendido e entregue por: KaBuM!".
- **[KBM-10]** https://www.kabum.com.br/busca/rtx-5070 — search page filters/sorting ("Relevância", "Vendido por", "Frete grátis", "OpenBox", "Prime Ninja").
- **[KBM-11]** https://www.kabum.com.br/esporte-e-lazer/bicicleta-eletrica — e-bike category exists (no vehicles).
- **[KBM-12]** https://www.kabum.com.br/ — home page: menu, "Venda no KaBuM!", Libras translator, no language/currency switcher.
- **[KBM-13]** https://blog.bling.com.br/kabum-marketplace/ — secondary source (2025-09-12): "18% do valor dos produtos vendidos + frete", no registration/monthly fee, CNPJ mandatory, 5-business-day shipping deadline from label issuance.
- **[KBM-20]** https://www.kabum.com.br/hotsite/ads-kabum/ — KaBuM! Ads formats, self-service, auction pricing, credits, contact e-mail; no rates.
- **[KBM-22]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188150058899-O-que-%C3%A9-Seller-Score — Seller Score rules (daily, 30 days, >10 orders).
- **[KBM-23]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188158230419-Regras-de-Repasse-Financeiro — payout conditions.
- **[KBM-24]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188174891923 — conciliation, cycle 01–31 paid day 7, MagaluPay balances, NFS 75%/25%.
- **[KBM-25]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188087682707 — 6.5% refund fee, contract clause 10.2.1.
- **[KBM-30]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188113061139 — MagaluPay registration.
- **[KBM-33]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188109214739-%C3%89-permitido-Dropshipping — purchase invoice required, no used/imitation goods, immediate suspension for dropshipping.
- **[KBM-34]** https://kabumatendimento.zendesk.com/hc/pt-br/articles/51943436566931 — accepted payment methods.
- **[KBM-35]** https://www.kabum.com.br/hotsite/cartao/termos-condicoes.html — Cartão KaBuM! (BB/Visa) terms: 24x promotional, cashback caps, 1P only.
- **[KBM-37]** https://kabumatendimento.zendesk.com/hc/pt-br/articles/49275419573651-Como-comprar-usando-o-PIX — "Até 10% de desconto com o PIX", QR valid 20 minutes.
- **[KBM-45]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188113994643 — seller fraud procedure; payout still paid if rules met.
- **[KBM-47]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188125735059 — freight table by CEP/weight ranges.
- **[KBM-48]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188123348755 — how buyer-facing delivery quote is calculated.
- **[KBM-51]** https://ajuda.olist.com/marketplaces/como-funciona-o-envio-de-pedidos-pelo-kabum-entregas — secondary source (integrator): KaBuM! Entregas label flow.
- **[KBM-52]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188125962643 — seller free-shipping settings.
- **[KBM-53]** https://www.kabum.com.br/produto/1/prime-ninja-kabum — Prime Ninja official product page: R$ 5,99 SP, R$ 5,00 discount, 12 months, 4.7 stars/118 reviews, out of stock.
- **[KBM-57]** https://kabumatendimento.zendesk.com/hc/pt-br/articles/51943436300563 — Correios pickup 7 days; other carriers no pickup; tracking path.
- **[KBM-58]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188174116499 — reverse label is seller responsibility.
- **[KBM-60]** https://kabumatendimento.zendesk.com/hc/pt-br/articles/51943415646227 — 3 delivery attempts.
- **[KBM-61]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188101236627-O-que-%C3%A9-Match-por-EAN — single product sheet per EAN.
- **[KBM-63]** https://kabumatendimento.zendesk.com/hc/pt-br/articles/51943398878227-Como-identificar-produtos-3P — "Vendido e entregue por" label and seller ratings shown to buyers.
- **[KBM-70]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188097884563 — pre-publication catalog check.
- **[KBM-71]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188069883923 — product approval statuses; offer blocked until approved.
- **[KBM-72]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188085742483 — rejection causes, prohibited word list, category not accepted.
- **[KBM-73]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188109266067 — image criteria.
- **[KBM-75]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188070346899 — categorization error article (title only).
- **[KBM-76]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188105566099 — evaluation on days 1 and 15; score < 2 → 15-day suspension.
- **[KBM-77]** https://kabummarketplace.zendesk.com/hc/pt-br/articles/48188136188947 — brand protection process and penalties up to permanent block.

### Webmotors (WM)

- **[WM-1]** https://www.webmotors.com.br/vender-carro — seller landing: plans Economic/Plus/Premium, no commission, payment methods, 10x, moderation up to 1 business day, Santander P2P financing, transfer guidance, 33M visits.
- **[WM-2]** https://www.webmotors.com.br/vender-carro/lp/como-anunciar-na-webmotors/ — requirements (18+, 100 km), plan descriptions, Turbinar upgrade, pricing varies by city/state (published 2026-01-26).
- **[WM-5]** https://www.webmotors.com.br/wm1/noticias/anunciar-gratis-webmotors — no free ads; 20% Premium coupon (site only); 2025-02-12.
- **[WM-6]** https://ajuda.webmotors.com.br/hc/pt-br/articles/26239240598676-M%C3%A9todos-de-pagamento-de-um-an%C3%BAncio — Pix, card brands, 10x, validation after payment.
- **[WM-7]** https://ajuda.webmotors.com.br/hc/pt-br/articles/45788382637844-Por-que-isso-%C3%A9-necess%C3%A1rio — biometric identity validation rationale.
- **[WM-8]** https://ajuda.webmotors.com.br/hc/pt-br/articles/45788472293652-Como-funciona-o-processo — document capture + selfie liveness process.
- **[WM-9]** https://ajuda.webmotors.com.br/hc/pt-br/articles/8913106477588-O-que-%C3%A9-Super-Pre%C3%A7o — Super Preço = 1-15% below FIPE, free.
- **[WM-11]** https://ajuda.webmotors.com.br/hc/pt-br/articles/360038764471-Como-cancelar-meu-an%C3%BAncio — 7-day withdrawal, max 2 per 12 months, sanctions for abuse.
- **[WM-12]** https://ajuda.webmotors.com.br/hc/pt-br/articles/10428123717268-Minha-Garagem — seller dashboard, chat, Turbinar, +Fotos.
- **[WM-13]** https://www.webmotors.com.br/financiamento — Santander simulator, up to 60x, 30-day coupon, store vs private flows.
- **[WM-14]** https://ajuda.webmotors.com.br/hc/pt-br/articles/7560085388692-Em-quanto-tempo-o-vendedor-recebe-o-valor-financiado-ap%C3%B3s-a-transfer%C3%AAncia-do-ve%C3%ADculo — payout to seller within 2 business days, own-name account only.
- **[WM-15]** https://www.webmotors.com.br/solutions/seguranca — security tips, chat, cartório transfer, report button, 2FA.
- **[WM-17]** https://ajuda.webmotors.com.br/hc/pt-br/articles/360038764411-Qual-o-prazo-de-reembolso — refund via card issuer up to two statements.
- **[WM-19]** https://ajuda.webmotors.com.br/hc/pt-br/articles/360038764011-Qual-%C3%A9-a-forma-mais-segura-de-negociar-um-ve%C3%ADculo — chat as safest channel.
- **[WM-20]** https://ajuda.webmotors.com.br/hc/pt-br/articles/7560043977236-Como-%C3%A9-feito-o-pagamento-da-entrada — down payment direct to seller, no liability.
- **[WM-21]** https://ajuda.webmotors.com.br/hc/pt-br/articles/7560607582484-Quais-s%C3%A3o-as-etapas-para-vender-financiado — P2P financing steps, payment within 48 h.
- **[WM-23]** https://www.webmotors.com.br/carros/estoque — full filter list, seller-type counts, sort dropdown, sponsored listings, badges.
- **[WM-24]** https://ajuda.webmotors.com.br/hc/pt-br/articles/360041134472-O-que-%C3%A9-Cardelivery — CarDelivery by partner stores.
- **[WM-25]** https://ajuda.webmotors.com.br/hc/pt-br/articles/5527639408532-O-que-%C3%A9-o-Comparador — comparator up to 4 0 km models, free.
- **[WM-26]** https://www.webmotors.com.br/seguranca/politica-de-privacidade/ — Terms of Use and Privacy Policy (updated May 2026): 18+, suspension grounds, biometrics, CNPJ, jurisdiction, data shared with Santander and CAR Group.
- **[WM-27]** https://ajuda.cockpit.com.br/hc/pt-br/articles/360057783171-O-que-%C3%A9-o-Plano-Performance — Performance plan: monthly fee + per-lead (price varies with car price), unlimited ads.
- **[WM-28]** https://ajuda.cockpit.com.br/hc/pt-br/articles/360057304252-O-que-%C3%A9-o-Plano-Controle — Controle plan: fixed subscription + lead franchise, fixed lead price.
- **[WM-29]** https://ajuda.cockpit.com.br/hc/pt-br/articles/11569241761556-O-que-o-Plano-Start-oferece — Start: 30 ads, free leads up to BRL 35,000.
- **[WM-31]** https://ajuda.cockpit.com.br/hc/pt-br/articles/30047119022740-O-que-%C3%A9-o-An%C3%BAncio-Pop — POP for cars up to BRL 50,000.
- **[WM-42]** https://ajuda.cockpit.com.br/hc/pt-br/articles/39441590369812-O-que-%C3%A9-o-Super-Acelerador-Nitro — Nitro accelerator, price varies by location.
- **[WM-44]** https://en.wikipedia.org/wiki/CAR_Group — secondary source: CAR Group markets (Australia, South Korea, Chile, USA, Brazil majority stake).
- **[WM-46]** https://ajuda.cockpit.com.br/hc/pt-br/articles/15143828795028-O-que-%C3%A9-o-Vistoriado — Vistoriado benefits for dealers.
- **[WM-51]** https://istoedinheiro.com.br/webmotors-cria-plataforma-de-pagamento-especializada-em-carros — secondary source (2018-10-08): Autopago wallet, 250-item inspection, funds released after transfer.
- **[WM-52]** https://www.webmotors.com.br/catalogo/comparativo-de-carros — comparator page (4 slots), footer links incl. Multas e Débitos (Zapay), Webmotors Ads.
- **[WM-56]** https://ajuda.webmotors.com.br/hc/pt-br/articles/15144542282132-O-que-%C3%A9-o-Vistoriado — buyer view: history + 120+ item inspection, downloadable report.
- **[WM-57]** https://www.cockpit.com.br/ — Loja Certificada with 1-year warranty; Cockpit solutions list.
- **[WM-59b]** https://ajuda.webmotors.com.br/hc/pt-br/articles/11711078055316-An%C3%A1lise-e-aprova%C3%A7%C3%A3o-do-an%C3%BAncio — pre-publication analysis; charge only after approval.
- **[WM-60]** https://ajuda.webmotors.com.br/hc/pt-br/articles/11711318956564-An%C3%BAncio-n%C3%A3o-publicado — ads with pending issues not published.
- **[WM-61]** https://ajuda.webmotors.com.br/hc/pt-br/articles/32151714656276-Quais-palavras-ou-caracteres-n%C3%A3o-podem-ser-inseridos-no-campo-de-observa%C3%A7%C3%A3o — blocked content in description (>4 digits).
- **[WM-63]** https://ajuda.webmotors.com.br/hc/pt-br/articles/360057072511-Reativar-o-meu-an%C3%BAncio — 90-day/30-day auto-deactivation, free reactivation under 1 year.
- **[WM-66]** https://ajuda.cockpit.com.br/hc/pt-br/articles/360057304372-O-que-%C3%A9-um-lead — lead = call, proposal or financing simulation.
- **[WM-68]** https://ajuda.cockpit.com.br/hc/pt-br/articles/35409611122196-O-que-%C3%A9-limitador-de-leads — free lead limiter.
- **[WM-71]** https://www.webmotors.com.br/solutions/webmotors-servicos — Serviçoauto: vehicle debts in up to 12x, workshops, coupons.
- **[WM-73]** https://ajuda.webmotors.com.br/hc/en-us — English locale request serves pt-br help center only.
- **[WM-75]** https://ajuda.webmotors.com.br/hc/pt-br/articles/10751154618132-Quanto-custa-vender-na-Webmotors — plans, no commission, prices vary by region.

### eBay (EB)

- **[EB-1]** https://www.ebay.com/help/selling/fees-credits-invoices/selling-fees?id=4822 — FVF by category, per-order fee, insertion fees, Below Standard/Very High surcharges, international fee 1.65%, currency conversion 3%, dispute fee $20, country fee-page links.
- **[EB-2]** https://www.ebay.com/help/selling/fees-credits-invoices/store-selling-fees?id=4809 — Store tiers and prices, free listing allocations, Store FVF (electronics 9.35%, most categories 12.7%).
- **[EB-3]** https://www.ebay.com/help/selling/selling/start-selling-ebay?id=4081 — eligible seller countries (Brazil listed), verification steps.
- **[EB-4]** https://www.ebay.com/help/selling/fees-credits-invoices/international-fees-ebay-global-sellers?id=5224 — international fee and currency conversion by region for global sellers (Latin America/Brazil).
- **[EB-5]** https://www.ebay.com/help/selling/shipping-items/setting-shipping-options/ebay-international-shipping-sellers?id=5348 — eBay International Shipping mechanics, protections, eligibility, destination table incl. Brazil.
- **[EB-6]** https://pages.ebay.com/br/pt-br/international-shipping/ — pt-BR landing page for eBay International Shipping; Brazil listed.
- **[EB-7]** https://community.ebay.com/t5/Shipping/eBay-International-Shipping-No-Longer-Shipping-to-Brazil/td-p/34494545/ — community thread quoting eBay support on Brazil restriction (secondary source).
- **[EB-8]** https://www.ebay.com/help/selling/fees-credits-invoices/motors-fees?id=4127 — vehicle listing packages/prices, no FVF, deposit processing fee 2.8%, deposit rules, dealer subscriptions.
- **[EB-9]** https://www.ebay.com/help/selling/ebay-advertising/promoted-listings/general-campaign-strategy?id=4164 — Promoted Listings General ad rate 2%–100%, 30-day attribution, eligibility.
- **[EB-10]** https://export.ebay.com/en/services-tools/advertising/priority-campaign-strategy/ — Promoted Listings Priority CPC, budgets, keyword targeting.
- **[EB-11]** https://export.ebay.com/en/growth/seller-performance/top-rated-seller/ — Top Rated requirements (100 transactions/$1,000, 90 days), Top Rated Plus (1-day handling, 30-day free returns, 10% FVF discount).
- **[EB-12]** https://www.ebay.com/help/selling/selling-tools/promoted-listings?id=4792 (resolves to seller registration/verification article) — KYC data (SSN/EIN, ID upload), bank account name match, payout hold until verified.
- **[EB-13]** https://www.ebay.com/help/selling/getting-paid/getting-paid-items-youve-sold?id=4814 — payout timing 1–2 days + 1–3 business days, schedules, checking account only, $2.00 express payout.
- **[EB-14]** https://www.ebay.com/help/selling/listings/selling-limits?id=4107 — selling limits exist, monthly review; no number published.
- **[EB-15]** https://nifty.ai/post/ebay-selling-limits — "around 10 items and $500" new-seller limit (secondary source, dated April 1, 2026).
- **[EB-16]** https://www.ebay.com/help/selling/getting-paid/payment-holds?id=4816 — hold durations for new private/business sellers, occasional sellers, high-priced items.
- **[EB-17]** https://www.ebay.com/help/buying/paying-items/paying-items?id=4009 — buyer payment methods, Amex not accepted, vehicle/Escrow.com exceptions.
- **[EB-18]** https://ebay.com/help/buying/paying-items/paying-klarna?id=5338 — Klarna $35–$30,000, Pay in 4 interest-free, financing 6–24 months at 7.99%–35.99% APR, US only.
- **[EB-19]** https://www.ebay.com/help/selling/getting-paid/handling-payment-disputes?id=4799 — 5-day response, 90-day hold, seller protection outcome.
- **[EB-20]** https://www.ebay.com/help/policies/selling-policies/payment-dispute-seller-protections?id=5293 — protection eligibility (delivered tracking, timely response), fee waiver.
- **[EB-22]** https://www.ebay.com/help/selling/shipping-items/shipping-rates/shipping-discounts?id=4168 — eBay discounted rates with USPS, FedEx, UPS; passing savings to buyers.
- **[EB-23]** https://pages.ebay.com/promo/2020/1022/Shippinglabel.html — 2020 promo: up to 48%/74% FedEx-UPS discounts, 20% FVF back, automatic tracking (historical).
- **[EB-27]** https://www.ebay.com/help/selling/shipping-items/setting-shipping-options/local-pickup?id=4181 — pickup code confirmation and INR protection.
- **[EB-28]** https://www.ebay.com/sellercenter/listings/item-specifics — item specifics as buyer filters; required vs recommended.
- **[EB-29]** https://www.valueaddedresource.net/ebay-item-compare-in-search/ — Item Compare: up to 3 listings, electronics, Feb 2026 (secondary source).
- **[EB-30]** https://export.ebay.com/en/grow-your-business/product-based-shopping-experience/ — product pages grouping listings by catalog product; unique items exempt.
- **[EB-32]** https://pages.ebay.com/br/ — HTTP 404 "Looks like this page is missing" (curl check); no Brazilian portal.
- **[EB-33]** https://www.ebay.com/help/selling/listings/product-reviews?id=5186 — product reviews from purchasers, 5-star average, catalog matching via UPC/EAN/MPN.
- **[EB-34]** https://www.ebay.com/help/buying/resolving-issues/leaving-feedback-sellers?id=4007 — feedback scoring (+1/0/−1), percentage, 60-day window.
- **[EB-35]** https://www.ebay.com/help/buying/resolving-issues-sellers/seller-ratings?id=4023 — Detailed Seller Ratings criteria (1–5 stars), Top Rated Plus seal.
- **[EB-37]** https://www.ebay.com/help/policies/selling-policies/seller-performance-policy?id=4347 (also served as seller-standards-policy?id=4347) — 20th-of-month evaluation, 3/12-month lookback, Above Standard 2%/0.3%, Top Rated 0.5%/0.3%/3%/95%, Below Standard consequences.
- **[EB-39]** https://www.ebay.com/help/selling/leaving-feedback-buyers/leaving-feedback-buyers?id=4078 (and pt slug como-dar-feedback-para-compradores?id=4078) — "you can only leave positive feedback for buyers"; page renders in English.
- **[EB-40]** https://www.ebay.com/help/buying/resolving-issues-sellers/contacting-seller?id=4021 — Contact seller flow, response-time display, no off-site sales.
- **[EB-41]** https://www.ebay.com/help/policies/prohibited-restricted-items/prohibited-restricted-items?id=4207 — prohibited/restricted list, enforcement actions, notification.
- **[EB-42]** https://www.ebay.com/help/policies/listing-policies/listing-policies?id=4213 — listing policy enforcement, removal email, Seller Help for issues.
- **[EB-43]** https://help.3dsellers.com/en/articles/15857677-ebay-listing-policies-what-s-allowed-what-s-not-and-troubleshooting — wrong-category handling (secondary source, search snippet only).
- **[EB-44]** https://www.ebay.com/help/account/account-holds-restrictions-suspensions/account-holds-restrictions-suspensions?id=4190 — restriction/suspension reasons, notice by email, reinstatement.
- **[EB-46]** https://www.ebay.com/help/selling/selling/selling-vehicles-parts-accessories?id=4647 — VIN/AutoCheck, title disclosure, deposit ≥1%, formats.
- **[EB-47]** https://pages.motors.ebay.com/sell/howto/complete-finalize.html — finalize sale: payment terms set by seller, pickup, title transfer per state DMV.
- **[EB-48]** https://www.escrow.com/partners/landing/ebaymotors — Escrow.com payment methods, 2-day inspection, payout timing.
- **[EB-49]** https://pages.ebay.com/secure-purchase/ — eBay Secure Purchase (Caramel): $25 buyer fee, payment/financing/title handling, timelines, eligible vehicles.
- **[EB-50]** https://pages.ebay.com/ebaymotors/buy/purchase-protection/index.html — Vehicle Purchase Protection up to $500,000 (from Aug 1, 2026), coverage, conditions, 45-day claim window.
- **[EB-52]** https://community.ebay.com/t5/Buying/How-to-disable-currency-conversion-in-search-results/td-p/33554909 — currency "approximate" conversion and Site Preferences (secondary source, search snippet only).
- **[EB-53]** https://www.ebay.com/help/selling/selling/selling-internationally?id=4132 — international shipping vs listing on other sites; international fee; no Brazil mention.
