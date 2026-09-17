# Payment Providers

> **Status:** research document supporting `docs/requirements.md`, sections 11 (Payments), 25 (External integrations) and the open topic "Payment gateway selection" in section 29. It records what each provider publishes; it does not decide. The closing sections give a shortlist and suggestions for the owner to accept or reject.
>
> **Access date for every source:** 2026-09-17. Each fact carries a reference such as `[MP-3]`; the [Sources](#sources) section gives the URL and what the page supports.
>
> **Legend:**
> - *Official* means a page published by the provider (pricing page, developer documentation, terms, help center).
> - *(secondary)* marks facts from press, consultants or integrators, quoted because the official page was unreachable or silent.
> - `not found` means the data point was searched for and not located; nothing is estimated. "On request" means the official page itself says the value is negotiated.
> - Amounts are in BRL unless stated. Percentages apply to the transaction amount unless the cell says otherwise.

## Scope and method

What the platform needs from the gateway (from `docs/requirements.md`): checkout inside the platform; PIX and credit card with installments; split payments with a sub-account per third-party store, onboarded and KYC-verified through the gateway; payout to stores held until delivery is confirmed; card tokenization so the platform never stores card data; webhooks translated into internal events; lowest possible fixed cost; a data model that lets a second gateway coexist later; and, for future markets, other countries and currencies. The vehicles marketplace may need a reservation or deposit held for high values.

Providers compared:

| Code | Provider | Why it is here |
|---|---|---|
| MP | Mercado Pago | Largest Brazilian wallet and acquirer; native marketplace split with OAuth onboarding |
| PGM | Pagar.me (Stone) | PSP built for marketplaces: recipients, split rules, anticipation |
| ASA | Asaas | Self-service PSP with white-label sub-accounts and split, low fixed cost |
| IUG | Iugu | Subscription and marketplace PSP with sub-accounts and split |
| PGB | PagBank (PagSeguro) | Large acquirer with "Connect" OAuth model for marketplaces |
| STR | Stripe (Connect) | International reference; Go SDK; multi-country platform model |
| ADY | Adyen (for Platforms) | Enterprise reference; single API across countries; balance platform |

Method: seven parallel research passes, one per provider, reading official pricing pages, developer documentation, terms and help centers directly. Where a provider only publishes "on request", that is recorded as the finding.

Dimensions: 1 pricing, 2 marketplace split and sub-accounts, 3 seller onboarding and KYC, 4 payment methods, installments and tokenization, 5 settlement and payouts, 6 disputes, refunds and anti-fraud, 7 developer experience, 8 compliance, 9 international readiness. Section 10 scores fit against the requirements and gives a shortlist; section 11 lists implications for `docs/requirements.md`.

---

## 1. Pricing

| | PIX | Credit card (à vista / installments) | Boleto | Fixed costs and plans | Marketplace-specific and other fees |
|---|---|---|---|---|---|
| **MP** | 0,99% "na hora" (Checkout / Link); 0,49% via presential QR product [MP-1][MP-3] | 3,98% (30 days) / 4,49% (14 days) / 4,98% ("na hora", after sales-history review). Installment ("parcelamento") rates only inside the account: on request [MP-1][MP-10] | R$ 3,49, up to 3 days [MP-1] | No monthly fee, no setup fee ("não cobramos mensalidade"); Pix and TED transfers out free [MP-7][MP-6] | No per-seller fee (sellers hold their own free accounts). Chargeback fee, refund fee, anti-fraud, 3DS and vault fees: `not found`. Anticipation: rate shown in-app before each operation, on request [MP-8][MP-40] |
| **PGM** | 0,99% (Essencial plan) [PGM-3] | 4,19% à vista; 6x = 13,63% (only tier published); "até 21x"; other tiers `not found` [PGM-3] | Per-boleto fee "percentual ou taxa fixa, a depender do seu contrato": on request [PGM-21] | Essencial: "Mensalidade grátis", processing and Stone anti-fraud "sem custos"; split only in the Flex plan: "TAXAS CUSTOMIZADAS", sales contact [PGM-3] | Per-recipient fee `not found`; TED payout fee "acordado no momento da contratação" (a 2017 API example shows R$ 3,67); PIX/boleto/anti-fraud/gateway fees not refunded on refunds, MDR refunded proportionally [PGM-21][PGM-17][PGM-22] |
| **ASA** | R$ 1,99 flat per received PIX (promo R$ 0,99 for 3 months) [ASA-1] | R$ 0,49 + 2,99% à vista; 2–6x 3,49%; 7–12x 3,99%; 13–21x 4,29% (percent on the total sale; R$ 0,49 once per charge) [ASA-1] | R$ 1,99 flat [ASA-1] | "Não há mensalidade ou taxa de adesão"; anti-fraud included free [ASA-1] | Sub-account creation "pode gerar cobranças de taxas específicas": on request; "Conta Escrow" monthly fee per enabled sub-account: on request; PIX transfers out: PJ 30 free/month then R$ 2,00, TED R$ 5,00; anticipation 1,25% a.m. (à vista) / 1,70% a.m. (installments); chargeback fee `not found`; fees not returned on refunds [ASA-8][ASA-12][ASA-1][ASA-24] |
| **IUG** | 0,99% (Essencial) [IUG-2] | 3,34% à vista; 2–6x 4,28%; 7–12x 4,79% (max 12x); plus R$ 0,40 processing fee per card or boleto transaction [IUG-2] | R$ 2,19 + R$ 0,40 [IUG-2] | Plan prices "Consulte" (sales-quoted); FAQ: "cobrados em modelo de assinatura e cobranças mensais", possible setup fee, 30-day cancellation notice. (secondary) cheapest plan R$ 149/month [IUG-1][IUG-2][IUG-7] | Split requires the "Essencial + Split" plan: R$ 1,00 "manutenção subconta" and R$ 2,50 per sub-account withdrawal; anticipation "consulte"; MDR charged again on refunds; chargeback fee `not found` [IUG-2][IUG-24][IUG-16] |
| **PGB** | 1,89% online ("limitada a 1,89%"), settled "na hora"; 0,99% on the hosted Link de Pagamento [PGB-1][PGB-6][PGB-5] | 4,99% + R$ 0,40 (payout 14 days) or 3,99% + R$ 0,40 (30 days) à vista; seller-funded interest-free installments cost "2,99% ao mês"; online per-installment table `not found` (Link table: 2x 8,52% up to 12x 21,20%, buyer-paid) [PGB-1][PGB-2][PGB-5] | 4,99% + R$ 0,40 (14 days) / 3,99% + R$ 0,40 (30 days) [PGB-2] | "Sem aluguel, mensalidade nem taxa de adesão"; "Split de Pagamentos PagBank é pago? Não"; but marketplace transaction rates are "de acordo com o que você negociar com o PagBank" [PGB-3][PGB-11] | Per-sub-account fee, chargeback fee, refund-fee policy: `not found`; TED and PIX out free and unlimited; anticipation rate on request (credit analysis); anti-fraud included [PGB-1][PGB-8][PGB-11] |
| **STR** | 1,19% per paid PIX, "somente por convite" (invite only) [STR-2] | 3,99% + R$ 0,39 domestic cards (pricing page) or "a partir de 3,99% + R$ 0,50" (Connect page; both quoted); +2% international cards; installments for Brazil: `not found` on official pages [STR-2][STR-3][STR-11] | R$ 3,45 per paid boleto [STR-2] | No setup or monthly fee on Standard [STR-2] | Connect, platform-managed pricing: R$ 6 per active account per month + 0,25% + R$ 0,67 per payout; Stripe-managed pricing: no platform fee; KYC included. Disputes: R$ 55 received (kept) + R$ 55 to counter (returned if won). Radar anti-fraud: Lite included, Standard R$ 0,25 / Plus R$ 0,35 / Pro R$ 0,45 per transaction. Processing fees not returned on refunds [STR-3][STR-16][STR-17][STR-15] |
| **ADY** | Fixed fee (USD 0,13 / EUR 0,11 shown, no BRL) + volume-based, "contrato direto": on request [ADY-1][ADY-2] | Fixed fee + Interchange++ + 0,60% (global row; Brazil-specific rate `not found`); installment tiers `not found` [ADY-1][ADY-2] | Fixed fee + volume-based: on request [ADY-1][ADY-2] | No monthly, setup, integration or closure fees, but "uma fatura mínima, dependendo do setor ou do modelo empresarial": amount on request [ADY-1][ADY-2] | Fees can be booked to the seller's balance account via split types (AcquiringFees, AdyenFees, PaymentFee); per-account-holder fee, chargeback fee, refund fee, payout fee: `not found` [ADY-12] |

**Observations**

- Two pricing postures: self-service with published rates and no fixed cost (Mercado Pago, Asaas, Stripe Standard, Pagar.me Essencial) versus sales-quoted (Iugu plans, Pagar.me Flex, Adyen with a minimum invoice). For a platform that must scale to zero, the second group carries a recurring cost before the first sale.
- The marketplace feature itself is where the fixed costs hide: Iugu charges R$ 1,00 per sub-account and R$ 2,50 per withdrawal; Stripe charges R$ 6 per active connected account plus a payout fee; Asaas charges an escrow fee per sub-account on request; Pagar.me moves split customers to the negotiated Flex plan.
- PIX pricing differs in kind: percentage (0,99% at Mercado Pago, Pagar.me and Iugu; 1,19% at Stripe) versus flat (R$ 1,99 at Asaas). For a vehicles marketplace with high tickets, a flat PIX fee is a different cost structure from a percentage one.
- Installment pricing is published only by Asaas and Iugu as tiers; Pagar.me publishes a single 6x example (13,63%, on the total); Mercado Pago publishes only a term-based rate and hides the installment surcharge inside the account; Stripe and Adyen publish nothing for Brazilian installments.
- Refunds are never free of the original fees: every provider keeps the processing fee or at least the fixed fees (Asaas, Pagar.me, Stripe, Iugu re-charges MDR). This matters for the commission-refund rule discussed in the competitive analysis.

## 2. Marketplace split and sub-accounts

| | Product and sub-account model | Split rules | Hold / escrow until delivery | Refunds and chargebacks on split orders |
|---|---|---|---|---|
| **MP** | "Split de Pagamentos 1:1": each seller has its own Mercado Pago account and authorises the marketplace via OAuth (token valid 6 months); no API-created sub-accounts; marketplace account needs KYC level 6; 1:N split only for "carteira assessorada" via sales [MP-12][MP-13][MP-14] | One fixed BRL commission per payment (`marketplace_fee` / `application_fee`); percentage `not found`; Mercado Pago fee deducted from the seller first; split moves money only between Mercado Pago accounts [MP-15] | `not found` as a marketplace-controlled feature: release follows the seller's own term (na hora / 7 / 14 / 30 days); marketplace-fee release date only via sales contact; MP retains disputed amounts itself [MP-5][MP-13][MP-34] | Refund split proportionally between seller and marketplace; the marketplace "não poderá realizar o reembolso total se o vendedor não tiver dinheiro na conta". Chargeback allocation on split: `not found` [MP-15] |
| **PGM** | "Recebedores": created via API, one per store, PF or PJ, each with a free "Conta Digital" under Stone Pagamentos (BACEN-regulated); marketplace "saldo global" is the sum of recipients; split is "disponível apenas para clientes PSP" (Flex plan) [PGM-8][PGM-12][PGM-24][PGM-6] | `flat` or `percentage` per recipient, N recipients per payment, per recipient `liable` (chargeback), `charge_processing_fee`, `charge_remainder_fee`; fixed anti-fraud and gateway fees only chargeable to the marketplace; rounding and per-item split `not found` [PGM-6][PGM-23] | No named escrow. `transfer_enabled=false` disables automatic payouts and the platform releases with `POST /transfers`; recipients in `registration`/`affiliation` can transact but not withdraw. Max hold `not found` [PGM-8][PGM-17][PGM-7] | `liable` marks who bears the chargeback; negative recipient balances offset the global balance; refund or chargeback enters the payables agenda as negative at D+2 business days [PGM-6][PGM-24][PGM-22] |
| **ASA** | "Subcontas" created via `POST /v3/accounts` by a CNPJ parent; PF or PJ holders; each sub-account has its own balance, API key and `walletId`; BaaS model needs account-manager enablement; regulatory evaluation period: max 10 sub-accounts, R$ 2.000 each, 60 days until homologation [ASA-8][ASA-20][ASA-27] | `fixedValue` and/or `percentualValue` per `walletId`, unlimited recipients, percentage on `netValue` (after Asaas fees, borne by the charge owner), remainder stays with the charge owner; per-installment values; `PAYMENT_SPLIT_DONE` webhook [ASA-10][ASA-11] | "Conta Escrow": per sub-account `daysToExpire`, automatic release at expiry or manual `POST /v3/escrow/{id}/finish`; stated use case "marketplaces que liberam o valor após a confirmação da entrega"; monthly fee per enabled sub-account (on request); no escrow webhook; max days `not found` [ASA-12][ASA-13][ASA-15] | Refund reverses the splits; `splitRefunds` lets the platform choose how much each participant returns; chargeback allocation and negative balance on split: `not found` [ASA-10][ASA-24][ASA-26] |
| **IUG** | "Marketplace": master account + "Subcontas" created via `POST /v1/marketplace/create_account` (production only, RSA-signed, cannot be deleted); PF or PJ; each sub-account is a prepaid payment account; the platform is "mandatário" and "solidariamente responsável" for sub-account chargebacks, MED and refunds [IUG-11][IUG-12][IUG-14][IUG-4] | `percent` and/or `cents`, per payment method and per installment (1x–12x), account default or per invoice, unlimited recipients via API; the invoice-creating account pays Iugu fees; sum must not reach 100%; per-recipient liability flag `not found` [IUG-9][IUG-10][IUG-17] | No escrow product. Primitives: card pre-authorisation (auto-cancel after 7 days), sub-account `disabled_withdraw` and `customer_minimum_balance_cents`, master-held funds released via `POST /v1/transfers` (min 1 cent). Max hold `not found` [IUG-18][IUG-20][IUG-23] | Refunds split proportionally; MDR charged again to the refunding account; refund fails if the account lacks balance; chargeback clawback on split `not found` (contract allows retention of receivables) [IUG-16][IUG-4] |
| **PGB** | "Split de Pagamentos" + "Connect": every receiver is a full PagBank account (`ACCO_`), created by the platform via `POST /accounts` (PF or PJ) or linked via OAuth; the seller holds its own balance; split restricted to single-seller carts; mandatory homologation before production (SLA 4 business days) [PGB-11][PGB-12-criar-conta][PGB-12-divisao][PGB-12-homologacao] | `FIXED` cents or `PERCENTAGE` (sum exactly 100, decimals allowed), per receiver `liable`, `custody`, `chargeback.charge_transfer`; primary may net zero; per charge, not per item [PGB-12-como-utilizar][PGB-12-liable] | "Custódia": `custody.apply = true` per receiver blocks its share; automatic release after 90 days by default, up to 365 days via `custody.release.scheduled`; manual release `POST /splits/{id}/custody/release` by the primary (500 requests per 5 minutes) [PGB-12-custodia][PGB-12-liberar-custodia] | Cancellation refused if any receiver lacks balance (no negative balances); refund proportional or customised with a fee-liable receiver; chargebacks debited 100% from the primary, recoverable from one secondary via `charge_transfer.percentage = 100` [PGB-12-cancelamento-split][PGB-12-recuperacao-chargeback] |
| **STR** | "Stripe Connect": connected accounts created via API (BR platform → BR accounts only), PF or PJ; charge types direct, destination (`application_fee_amount`) or separate charges and transfers; BR is listed with capabilities card, transfers, boleto and PIX, although the legacy account-types page omits BR from Express/Custom lists [STR-20][STR-4][STR-8] | Amounts computed by the platform; transfers to many accounts per `transfer_group`; no percentage primitive; fee payer depends on charge type (platform for destination/separate) [STR-4][STR-5] | "Stripe doesn't provide escrow services"; pattern: platform keeps funds and transfers later (separate charges and transfers), compliance holding limit 90 days; manual payout schedules are NOT available in Brazil ("Payouts are always automatic and daily") [STR-29][STR-30][STR-13] | Refunds and disputes debit the platform balance; recover via transfer reversal if the seller balance covers it; negative balances: Stripe holds a reserve on the platform; automatic bank debit not available in BR [STR-4][STR-5][STR-30] |
| **ADY** | "Adyen for Platforms": account holders and balance accounts created via Legal Entity Management and Configuration APIs; Brazil availability conflicting: absent from the onboardable-countries lists, present in the verification-requirements table; confirm with sales [ADY-3][ADY-4][ADY-9][ADY-18] | `splits` array with fixed amounts per balance account; types BalanceAccount, Commission, PaymentFee, AcquiringFees, AdyenFees, Remainder; percentage `not found`; max splits `not found` [ADY-11][ADY-12] | Funds stay in the seller's balance account until a scheduled "push sweep" or an on-demand transfer, so the platform controls release; named hold feature and max period `not found` (payout pages 404) [ADY-20][ADY-21] | Refunds deducted by the original split ratio unless split instructions are sent; chargebacks on split: page exists, `not found` this session [ADY-22] |

**Observations**

- Three architectures exist. Wallet-linking (Mercado Pago: the seller's own account plus OAuth) gives no platform control over release. Recipient/sub-account APIs (Pagar.me, Asaas, Iugu, Stripe, Adyen) create the store's account from the platform and let the platform decide when money moves. Only Asaas names an escrow product with a delivery-confirmation use case; the others offer primitives (disable automatic payouts, transfer on demand).
- Percentage splits computed by the gateway exist at Pagar.me, Asaas and Iugu; Stripe and Adyen take amounts the platform computes. Since the requirements already store money in minor units and will compute commission themselves, an amount-based split is sufficient, but per-recipient liability flags (Pagar.me `liable`) simplify chargeback handling.
- Liability is contractual and shifts to the platform: Iugu's contract makes the master jointly liable for sub-account chargebacks and refunds; Stripe debits the platform balance for destination and separate charges; Pagar.me requires at least one recipient to own chargebacks and fees.
- Stripe's Brazilian restriction (no manual payout schedule) forces the hold onto the platform balance, with a 90-day compliance limit. Mercado Pago's 1:1 model cannot express "hold until delivery" at all without a commercial agreement.

## 3. Seller onboarding and KYC through the gateway

| | Onboarding channel | Data and documents | Who bears KYC, approval time, status | Limits for new stores and payout destination |
|---|---|---|---|---|
| **MP** | OAuth link to the seller's existing Mercado Pago account (code valid 10 minutes; token 6 months, then refresh); no API account creation [MP-14][MP-13] | Collected by Mercado Pago in its own flow (ID scan + face); Brazil residents with CPF or CNPJ only; foreign sellers not allowed [MP-17][MP-11] | Mercado Pago as BACEN-regulated institution; approval time and onboarding status webhooks `not found` [MP-11][MP-13] | New sellers start with 7/14/30-day release; "na hora" after sales-history review; caps `not found`. Funds stay in the seller's Mercado Pago account; Pix/TED out free [MP-5][MP-6] |
| **PGM** | `POST /core/v5/recipients` with `register_information` (mandatory since 29 Feb 2024) plus biometric KYC link (`POST /recipients/{id}/kyc_link`, QR valid 20 minutes) [PGM-8][PGM-11] | PJ: CNPJ, revenue, address, phones and exactly one managing partner from the QSA (CPF, income, occupation); PF: CPF, birthdate, income, occupation, address; `default_bank_account` at any bank; foreigners may present CRNM/RNE [PGM-9][PGM-12] | Pagar.me validates; result within 24 h via `recipient.updated` (approved / refused / redo); statuses registration → affiliation → active / refused / suspended / blocked [PGM-11][PGM-7][PGM-16] | Recipients can transact but not withdraw until active; new sellers may be held up to 15 days; caps `not found`. Payout to the seller's own bank account (TED); no Pagar.me account needed by the seller [PGM-7][PGM-3][PGM-26] |
| **ASA** | `POST /v3/accounts` (parent must be CNPJ) returns the sub-account API key and `walletId`; documents via `onboardingUrl` or documents API; BaaS model enabled by the account manager [ASA-20][ASA-21][ASA-9] | name, e-mail, CPF/CNPJ, phone, monthly income, address; PF birthdate or PJ company type; ID + selfie (liveness) [ASA-20][ASA-22] | Asaas analyses within 48 h (liveness usually 5 minutes); `GET /v3/myAccount/status` and `ACCOUNT_STATUS_*` webhooks; annual re-confirmation of commercial data [ASA-22][ASA-23] | Regulatory evaluation period for the integration: 10 sub-accounts, R$ 2.000 in charges each, 60 days until homologation; foreign sellers `not found`. Seller holds balance in its sub-account; transfers to any bank by PIX or TED [ASA-27][ASA-28] |
| **IUG** | `POST /v1/marketplace/create_account` then `request_verification` within 24 h of creation, one call only (production only, RSA-signed) [IUG-11][IUG-14] | price range, business type, PF (CPF, name) or PJ (CNPJ, company name, responsible CPF), address, phone, revenue, bank account, PEP flag; documents: ID, selfie, social contract for PJ, proof of address [IUG-14][IUG-12] | Contract puts document collection and risk analysis on the platform ("A iugu não será responsabilizada nos casos de Subcontas abertas sem a devida verificação"); Iugu answers in up to 2 business days; webhooks `referrals.verification` with `charge_limit_cents`, `document_status_change`, `bank_verification` [IUG-4][IUG-14][IUG-21] | Unverified sub-accounts get 401 on live tokens; `charge_limit_cents` returned per account (tiers `not found`); withdrawals only to a same-holder bank account, D+1, minimum R$ 5,00 [IUG-14][IUG-21][IUG-23] |
| **PGB** | `POST /accounts` (type SELLER, PF or PJ, `business_category` including VEHICLE_AND_PARTS) or Connect OAuth (code valid 10 minutes; scopes for payments and account reads) [PGB-12-criar-conta][PGB-12-connect-auth] | name, birth date, mother's name, CPF, address, phones, terms acceptance with IP and date; company data for PJ [PGB-12-criar-conta][PGB-12-cadastro] | KYC against Receita Federal; the created account is "simples": to move funds the holder must complete "avanço de conta" in the PagBank app (document photo + selfie); approval time and status webhooks `not found` [PGB-12-cadastro] | Per-transaction limits "conforme políticas internas" (caps `not found`); seller holds balance in its PagBank account and withdraws free by PIX or TED [PGB-10][PGB-1] |
| **STR** | Hosted onboarding (Account Links), embedded component or full API (Accounts + Persons + Files) [STR-21] | BR individual: name, e-mail, phone, birth date (18+), address, CPF, PEP flag, proof of liveness, bank account, terms acceptance; BR company: CNPJ, address, representative with CPF and liveness, owners and directors required for the PIX capability [STR-20] | Stripe verifies ("KYC included"); minutes to a few business days for documents; `account.updated` webhooks with `requirements` and `disabled_reason`; BR platform can only onboard BR accounts [STR-21][STR-3][STR-10] | Caps `not found`; new private/business hold rules not BR-specific; payouts to any Brazilian bank account (bank code + branch + account); PIX-key payouts `not found` [STR-20][STR-18] |
| **ADY** | Hosted onboarding or API-only via the Legal Entity Management API; Brazil appears in the verification table but not in the onboardable-countries lists [ADY-9][ADY-18][ADY-3] | Individual, organization or sole proprietorship; CPF/CNPJ formats page 404 (`not found`) [ADY-18] | Adyen runs KYC before payouts; approval time and event names `not found` [ADY-4][ADY-23] | `not found`; payout to the user's transfer instrument via sweep; Brazilian PIX/TED specifics `not found` [ADY-20] |

**Observations**

- Two onboarding models: the store owns an account at the provider and links it (Mercado Pago, PagBank Connect), or the platform creates the store's account through the API (Pagar.me, Asaas, Iugu, Stripe, Adyen, PagBank accounts API). Only the second lets the platform show the whole onboarding inside its own console, which the requirements assume.
- KYC responsibility is not uniform. Pagar.me, Asaas and Stripe run the verification themselves and report a status; Iugu contractually shifts collection and risk analysis to the platform; PagBank requires the store to finish verification in PagBank's own app before it can move funds.
- Biometric liveness (selfie) is required by Pagar.me, Asaas, Stripe and PagBank. This matches the competitor practice noted in the competitive analysis and reduces the need for a separate platform-level identity check.
- Regulatory ramps exist: Asaas limits a new integration to 10 sub-accounts and R$ 2.000 each for 60 days; PagBank requires homologation; Pagar.me's split is only for PSP (Flex) customers. These are calendar items for the launch plan.

## 4. Payment methods, installments and tokenization

| | PIX | Cards and tokenization | Pre-authorization (vehicle reservation) | Installments |
|---|---|---|---|---|
| **MP** | QR + copy-and-paste; expiry default 24 h, configurable 30 minutes to 30 days; refunds full or partial within 180 days; Pix Automático `not found` [MP-17][MP-20] | Brands via API (Visa, Mastercard, Elo, Amex, Hipercard...); browser token single-use, expires in 7 days; vault via Customers/Cards API; 3DS 2.0 with liability shift; SAQ A with Bricks/Secure Fields [MP-18][MP-19][MP-24][MP-29] | `capture_mode: manual`, capture within 5 days, full capture only [MP-16] | `installments` field; seller-funded "parcelado vendedor" configured per tool; buyer-interest computation `not found`; minimum installment `not found` [MP-30][MP-10] |
| **PGM** | `expires_in` mandatory; QR and copy-and-paste; refunds full or partial within 90 days; split supported; sandbox PIX simulator cannot be used with split [PGM-18][PGM-19] | Visa, Mastercard, Elo, Amex, Hipercard; tokenizecard.js token, card wallet (`card_id`), network tokens (Visa/Mastercard), Zero Dollar Auth for verification; 3DS 2.1/2.2 [PGM-25][PGM-31][PGM-32][PGM-28] | `auth_only` / `pre_auth` (needs acquirer enablement); capture windows: 5 h at checkout, Visa vehicle-rental MCC 29 days; generic API window `not found` [PGM-20][PGM-33][PGM-34] | `installments` integer, "até 21x"; interest expression in API `not found`; minimum installment `not found` [PGM-3][PGM-20] |
| **ASA** | Dynamic QR with expiry; copy-and-paste; refunds full or multiple partial; Pix Automático supported [ASA-34][ASA-24] | 12 brands incl. Visa, Mastercard, Elo, Amex; tokenization `creditCardToken` (production enablement by account manager); API-side tokenization keeps the platform in PCI scope (no JS/hosted fields); 3DS enabled by support [ASA-35][ASA-18][ASA-17][ASA-67] | `authorizeOnly: true`, default 3 days, configurable 3 to 25 days for eligible accounts; partial capture `not found` [ASA-36][ASA-37] | Up to 21x Visa/Mastercard, 12x others; platform sets `installmentCount` and values (gateway does not compute buyer interest); minimum installment `not found` [ASA-36][ASA-40] |
| **IUG** | QR and copy-and-paste; expiry via due date + `expires_in`; refund full only, within 90 days; Pix Automático supported; MED index above 2% may suspend PIX [IUG-13][IUG-28][IUG-15][IUG-4] | Visa, Mastercard, Amex, Diners, Elo, Hipercard via iugu.js; single-use token; saved `payment_methods` shareable across sub-accounts when created in the master; 3DS 2.0 marketed, API docs `not found` [IUG-31][IUG-33][IUG-34][IUG-3] | `two_step_transaction`: authorise, then capture or cancel; auto-cancel after 7 calendar days [IUG-18][IUG-19] | `months` 2–12; `max_installments_without_interest` then interest charged to the buyer; rate formula `not found` [IUG-20][IUG-35] |
| **PGB** | `pix.expiration_date`; single-use QR with risk analysis; refund "em poucos minutos", window 90 days; Pix Automático via API `not found` [PGB-12-pix-v2][PGB-7] | Visa, Mastercard, Amex, Elo, Hipercard; `card.encrypted` (public-key JS) for non-PCI integrators; `card.store: true` returns `card.id`; `POST /tokens/cards` validates and stores without charge; network tokens; 3DS via JS SDK [PGB-12-criar-pedido][PGB-12-tokens-cards][PGB-12-3ds] | `capture: false` reserves 6 to 29 days by brand (Visa, Mastercard, Elo 29; Amex, Hipercard 6); partial capture allowed; new splits allowed at capture [PGB-12-criar-pedido][PGB-12-capturar] | `installments` required for credit; Fees API computes buyer interest and seller fees per plan (`max_installments_no_interest`); minimum installment R$ 5 [PGB-12-fees][PGB-12-repasse] |
| **STR** | Invite only in Brazil; one-time only; expiry 10 s to 3 days (default 4 h); refunds within 90 days; limit BRL 0,50 to USD 3.000 equivalent [STR-2][STR-11][STR-25] | Visa and Mastercard credit only (no Amex, Elo, Hipercard, no local debit); Elements / Payment Element; SetupIntents and `setup_future_usage`; PCI Level 1 vault; Google/Apple Pay [STR-23][STR-12][STR-19] | `capture_method: manual`, 7-day window; extended authorization 30 days for Visa/Mastercard (IC+ pricing or support); partial capture [STR-14][STR-28] | Installments for Brazil `not found` on official pages [STR-11][STR-23] |
| **ADY** | QR default 24 h (v72+), configurable; copy-and-paste; refunds within 90 days; requires a Brazilian local entity [ADY-5][ADY-6] | Brazilian brand list `not found`; Adyen Vault tokens and network tokens; Sessions flow qualifies for SAQ A [ADY-10] | `authorisationType: PreAuth`; default 28 days; Visa 10, Mastercard 30, Amex 7 days; manual or delayed capture [ADY-7][ADY-24] | `installments.value` 1 to 99; interest added by the merchant to the amount; each installment settles with a 30-day delay [ADY-8] |

**Observations**

- Card brand coverage is a real differentiator in Brazil: Stripe accepts only Visa and Mastercard credit; the Brazilian providers accept Elo, Hipercard and Amex, which competitors' checkouts all list.
- Every provider except Asaas offers browser-side tokenization that keeps the platform at PCI SAQ A. Asaas tokenizes on the server side, which keeps the platform "in scope" per its own documentation. This is a direct conflict with the PCI intent in section 11 of the requirements.
- Pre-authorization windows range from 3 days (Asaas default) and 5 days (Mercado Pago) to 29–30 days (PagBank, Adyen, Stripe extended). A vehicle reservation held for more than a week by card needs PagBank, Adyen or Stripe; otherwise the reservation must be a captured PIX or card payment held as balance.
- Buyer-paid interest is computed by the gateway only at PagBank (Fees API) and, in a simpler form, Iugu (`max_installments_without_interest`). Asaas, Adyen and Pagar.me expect the platform to compute installment values, which means the platform needs its own interest table if buyers pay interest.
- PIX Automático (recurring) exists at Asaas and Iugu; it is not needed for the MVP.

## 5. Settlement and payouts

| | Settlement terms | Payout to stores | Anticipation | Provider holds and reserves |
|---|---|---|---|---|
| **MP** | Card: na hora / 14 / 30 days chosen by the seller (7 days for sellers since 17 Jun 2024); PIX na hora; boleto up to 3 days [MP-1][MP-5] | Automatic per payment into the seller's Mercado Pago account; Pix/TED out free; API-triggered payouts `not found` [MP-15][MP-6] | In-app, rate shown before each operation [MP-8] | Disputed amounts retained until mediation ends; contractual right to block balances; rolling reserve `not found` [MP-11][MP-34] |
| **PGM** | Cards ~30 days (installments 31/61/91 days) unless anticipated; Essencial "a partir de 1 dia"; PIX immediate [PGM-13][PGM-3][PGM-39] | Automatic per recipient (daily, weekly, monthly; business days; gated by the marketplace global balance; disabled after 60 idle days) or manual `POST /transfers` (TED, idempotent) [PGM-5][PGM-17] | Automatic or spot per recipient with simulate and limits endpoints; rate on request [PGM-14][PGM-38] | New sellers may be held up to 15 days; reserve % `not found` [PGM-3] |
| **ASA** | Card D+32 (installments D+32, D+64...); PIX immediate; boleto same day if paid before 13h30 (secondary) [ASA-1][ASA-43] | Seller keeps balance in its sub-account; `POST /v3/transfers` to bank (PIX or TED), PIX key or internal `walletId`; scheduled and recurring transfers; withdrawal-validation webhook lets the platform approve or refuse each outgoing transfer [ASA-28][ASA-29][ASA-46] | 1,25% a.m. à vista, 1,70% a.m. installments, credited within 2 business days; API with simulation [ASA-1][ASA-44] | Balance blocks notified by webhook (MED, judicial, administrative); reserve % `not found` [ASA-47] |
| **IUG** | Card every 30 days per installment; PIX within 10 s; TED up to 3 business days [IUG-4] | Sub-account withdraws to its own bank account (D+1, min R$ 5, business days, R$ 2,50 on the Split plan) or automatic scheduled withdrawal; platform-triggered internal transfers `POST /v1/transfers` [IUG-23][IUG-26][IUG-2] | Manual (business days 9:00–16:00) or automatic; compound monthly interest, negotiated [IUG-24] | Contract lets Iugu constitute reserves and use receivables as guarantee "unilaterally" and change them "sem aviso prévio" [IUG-4] |
| **PGB** | Card 14 or 30 days (choosable; "na hora" for card-present); PIX na hora; debit 1 day [PGB-1][PGB-3] | Funds land in each receiver's PagBank account at settlement or custody release; sellers withdraw free; platform-triggered transfer API exists behind mTLS, details `not found` [PGB-12-custodia][PGB-12-ambientes] | On demand or scheduled, credit analysis, rate on request [PGB-8] | "Reserva Financeira" set unilaterally; payouts suspended while a dispute opened within 30 days is pending [PGB-10] |
| **STR** | Domestic cards 30 calendar days; international 5 days; PIX and boleto 2 business days; not choosable; anticipation `not found` [STR-13] | Payouts in Brazil "always automatic and daily"; platform transfers via `POST /v1/transfers`; payout fee 0,25% + R$ 0,67 (platform-managed pricing); Instant Payouts not in BR [STR-13][STR-3][STR-31] | `not found` [STR-27] | Reserves possible (`reserve_appeal`); BR percentages `not found` [STR-21] |
| **ADY** | Sales-day batches; delay varies by region and method; Brazil installments settle each with a 30-day delay; PIX transferred once a day [ADY-21][ADY-8][ADY-5] | Scheduled push sweeps or on-demand transfers from the seller's balance account; fees and schedule `not found` [ADY-20] | `not found` | Reversed settlement if funds not received within 30 days of capture; reserves `not found` [ADY-21] |

**Observations**

- Card settlement in Brazil is structurally ~30 days (Asaas D+32, Iugu 30 days per installment, Stripe 30 calendar days, Adyen 30-day delay, Pagar.me ~30 days). Faster terms cost money everywhere: Mercado Pago and PagBank price 14-day and "na hora" plans higher; Asaas charges 1,25–1,70% a month; Pagar.me and Iugu negotiate.
- The payout hold the requirements propose therefore costs nothing for card sales during the first month, because the money has not settled yet. It matters for PIX, which settles instantly everywhere: an escrow or "do not auto-transfer" primitive is what keeps PIX money from reaching the store before delivery.
- Providers that keep the store's money in a store-owned account (Mercado Pago, PagBank, Asaas, Iugu) leave withdrawal in the store's hands unless custody/escrow or a withdrawal block exists. Providers that keep it in the platform's balance until a transfer (Stripe, Pagar.me with automatic payouts disabled, Adyen sweeps) make the platform the custodian, with the liability that implies.
- Every contract reserves the right to hold funds or impose reserves unilaterally (Iugu, PagBank, Mercado Pago). The platform's own payout promise to stores must leave room for that.

## 6. Disputes, chargebacks, refunds and anti-fraud

| | Chargeback flow and fee | Refunds | Anti-fraud and 3DS |
|---|---|---|---|
| **MP** | Webhook topic `chargebacks`; 10 calendar days to submit delivery proof (max 10 files, 10 MB, one submission); resolution up to 120 days; fee `not found`; free Seller Protection Program for tangible goods after 10 sales, no proof needed with Mercado Envios [MP-22][MP-23][MP-41][MP-40] | Full or partial within 180 days; card to statement, other methods to the payer's account; fee return `not found` [MP-20] | Own system included; fraud alert webhook `stop_delivery`; 3DS 2.0 optional with liability shift [MP-21][MP-28][MP-24] |
| **PGM** | `charge.chargedback` (to become `chargeback.received`); Stone Disputes API with PDF evidence (2 MB per file, 10 pages); fee and deadlines `not found` [PGM-16][PGM-44][PGM-45] | Card within 180 days (cardholder sees it in up to 7 business days); PIX and boleto within 90 days; MDR refunded proportionally, other fees kept [PGM-22][PGM-33] | Stone anti-fraud "sem custos" on Essencial; "Cobertura de Fraude" product (price `not found`); 3DS 2.1/2.2 [PGM-3][PGM-21][PGM-28] |
| **ASA** | Chargeback object with 33 reason codes, dispute status and document deadline; webhooks; evidence via API (article unreachable); fee `not found` [ASA-26][ASA-50] | Full or partial; card up to 10 business days to appear; PIX multiple partials; fees not returned [ASA-24][ASA-25] | Included free; risk-analysis events; provider name `not found`; 3DS on request [ASA-1][ASA-45b][ASA-17] |
| **IUG** | Invoice status `chargeback`; `PUT /v1/chargebacks/{id}/contest` with up to 5 files, 10 pages, 8 MB; fee `not found` [IUG-43][IUG-45] | Card full or partial within 180 days (buyer credited in 30–60 days); PIX full only within 90 days; boleto none; MDR charged again [IUG-15][IUG-16] | Depends on plan; "antifraude integrado" marketed; provider `not found` [IUG-4][IUG-3] |
| **PGB** | `CHARGEBACK.CREATED/UPDATED` events; dispute with one PDF (4 MB, 18 pages) while `AWAITING_EVIDENCE`; per-case `due_date`; fee `not found`; debited 100% from the primary [PGB-12-chargeback][PGB-12-criar-disputa][PGB-12-recuperacao-chargeback] | `POST /charges/{id}/cancel` full or partial; API window 350 days (PIX 90); requires balance at every receiver; fee policy `not found` [PGB-12-cancelar][PGB-7] | Included with split; provider `not found`; 3DS via SDK [PGB-11][PGB-12-3ds] |
| **STR** | Dispute events; respond in 7–21 days; issuer decides in 60–75 days; R$ 55 received (kept) + R$ 55 countered (returned if won) [STR-16] | Full or partial; 5–10 business days; original fees not returned; PIX within 90 days [STR-15] | Radar Lite included; Standard/Plus/Pro R$ 0,25/0,35/0,45 per transaction; 3DS via Radar rules [STR-17][STR-34] |
| **ADY** | RFI → chargeback → pre-arbitration; response windows Visa 9/18 days, Mastercard 40; fee `not found` [ADY-13] | Full or partial after capture; up to 40 business days to reach the shopper; fee return `not found` [ADY-14] | Risk management product; inclusion and price `not found` [ADY-13] |

**Observations**

- Chargeback fees are published only by Stripe (R$ 55 + R$ 55). Every Brazilian provider leaves the fee to the contract, which is a line item to ask for in a commercial proposal.
- Evidence submission is API-driven at all providers; the requirements' audit trail and shipping tracking (platform label with tracking) are exactly what these flows ask for. Mercado Pago's protection program waives proof when its own shipping is used, a benefit the platform will not have with a third-party shipping provider.
- Refund windows are generous (180 days for cards at Mercado Pago, Pagar.me and Iugu; 350 days at PagBank) and PIX refunds are near-instant, so the Brazilian right of withdrawal is easy to honour technically. The cost is the non-refunded processing fee.

## 7. Developer experience

| | API and idempotency | Webhooks | Sandbox | SDKs, docs, status |
|---|---|---|---|---|
| **MP** | REST, Orders API (`/v1/orders`) with `X-Idempotency-Key`; 429 with Retry-After, numeric limits `not found` [MP-31][MP-16][MP-33] | HMAC-SHA256 `x-signature`; acknowledge within 22 s; retries every 15 minutes; topics for payments, orders, chargebacks, claims, fraud alerts [MP-27][MP-28] | Test seller/buyer accounts per country, test cards by holder name, PIX simulation page [MP-25][MP-26] | 11 official SDKs including Go (Go 1.23+); docs pt/es/en with "copy for LLMs"; status page 99,95% 90-day uptime; OpenAPI download `not found` [MP-37][MP-38][MP-39] |
| **PGM** | REST `api.pagar.me/core/v5`, Basic auth with secret key; `Idempotency-Key` on transfers (on orders `not found`); rate limits 100–700 per minute by endpoint [PGM-48][PGM-17][PGM-47] | Event catalogue (orders, charges, recipients, checkout); configurable retry count; signature verification `not found` [PGM-16][PGM-49] | Test keys; simulator cards; PIX simulator (≤ R$ 500 auto-paid, not usable with split); KYC test environment [PGM-19][PGM-51][PGM-52] | Java, C#, Node, PHP, Python, Ruby and "Golang (beta)"; SDK v7 for API changes effective 28 Aug 2026; docs pt-BR only; status page; OpenAPI per page [PGM-53][PGM-54][PGM-14] |
| **ASA** | REST v3, `access_token` header; request idempotency `not found`; quota 25.000 requests per 12 h, 50 concurrent GET [ASA-52][ASA-66] | Up to 10 webhooks per account; token header (no HMAC); only HTTP 200 accepted; 10 s timeout; documented back-off up to 15 retries then queue paused; 14-day retention [ASA-53][ASA-54][ASA-55] | Separate sandbox with automatic account approval; test cards; PIX simulation; 3DS challenge not testable [ASA-58][ASA-59][ASA-60] | Official SDK Java only (no Go); Postman and OpenAPI 3.0.1; changelog; status page `not found`; docs pt-BR with partial English [ASA-61][ASA-63][ASA-65] |
| **IUG** | REST `/v1`, Basic auth; RSA request signing mandatory for sub-account creation, withdrawals and transfers; `Idempotency-Key` valid 24 h on key endpoints; production rate limits `not found` [IUG-46][IUG-47][IUG-48] | ~45 events; form-urlencoded payload; `authorization` key you set; max 20 webhooks; manual redelivery; automatic retry policy and HMAC `not found` [IUG-49][IUG-53] | Test token, 50 requests per minute, 1.000 test invoices per day; sub-account creation and transfers not testable; PIX simulation `not found` [IUG-46][IUG-11] | PHP, Ruby, .NET, Java, Node, Python; no Go; changelog current to 1 Sep 2026; status page; pt-BR [IUG-55][IUG-57][IUG-58] |
| **PGB** | REST, amounts in cents; Bearer token per seller (Connect); idempotency header documented, name `not found`; custody release limited to 500 requests per 5 minutes [PGB-12-ambientes][PGB-12-idempotencia][PGB-12-liberar-custodia] | Per-order `notification_urls`; SHA-256 authenticity token or new ECDSA `x-payload-signature` (page updated 15 Sep 2026); event preferences; post-transaction events still in legacy XML; retry policy `not found` [PGB-12-webhooks][PGB-12-validacao][PGB-12-preferencia] | Sandbox portal; KYC auto-approved; test cards by brand; PIX and boleto simulator by amount; mandatory homologation, SLA 4 business days [PGB-12-cartoes][PGB-12-simulador][PGB-12-homologacao] | Browser JS SDK only; server SDK list and Go `not found`; OpenAPI embedded per endpoint (v4.1); docs Portuguese only; status page [PGB-12-llms][PGB-12-status] |
| **STR** | REST v1/v2, `Stripe-Account` header, `Idempotency-Key`; 100 requests per second live; date-based API versions [STR-35][STR-40][STR-36] | HMAC-SHA256 `Stripe-Signature`; retries up to 3 days with exponential back-off; 16 endpoints; Connect event scope [STR-33] | Multiple isolated sandboxes; Connect sandbox cannot link to connected-account sandboxes; BR test cards and PIX triggers [STR-37][STR-38][STR-25] | Official Go SDK (`stripe-go`), OpenAPI spec on GitHub; docs English with pt-BR locale; status page [STR-39][STR-41] |
| **ADY** | REST, versioned paths (v72), API key; `idempotency-key` valid 7–14 days; rate limits `not found` [ADY-27][ADY-29] | HMAC-signed events; retry and acceptance rules `not found` this session (pages 404) [ADY-23] | Test Customer Area; PIX and boleto simulated by "promote to sale" [ADY-28][ADY-6] | Official Go library v21 covering Checkout, Balance Platform, Transfers, Disputes; docs English [ADY-26][ADY-30] |

**Observations**

- Official Go SDKs exist at Stripe, Adyen, Mercado Pago and Pagar.me (beta). Asaas, Iugu and PagBank have none, which means writing the client against the OpenAPI definitions they embed. Given the ports-and-adapters rule in section 25, a hand-written client is acceptable but is extra work and extra tests.
- Webhook security varies from signed (Mercado Pago, Stripe, Adyen, PagBank new scheme) to a shared token in a header (Asaas, Iugu, Pagar.me undocumented). The internal event translation layer should verify signatures where available and, for token-based providers, rely on re-fetching the object from the API before acting.
- Sandbox gaps matter for the "fake implementations" testing rule: Iugu cannot create sub-accounts or transfer in sandbox; Pagar.me's PIX simulator does not work with split; Stripe cannot link platform and connected-account sandboxes. End-to-end split tests will need real accounts at these providers.

## 8. Compliance and security

| | PCI scope for the platform | BACEN status | LGPD and data residency |
|---|---|---|---|
| **MP** | SAQ A with Bricks / Secure Fields; legacy direct API = SAQ D [MP-29] | Instituição de pagamento in four modalities (e-money issuer, post-paid instrument issuer, acquirer, initiator) [MP-11] | Privacy statement; fraud-data sharing under Resolução Conjunta 6; data residency `not found` [MP-11] |
| **PGM** | tokenizecard.js keeps PAN off the platform; own PCI attestation `not found` [PGM-2][PGM-31] | Recipient accounts under Stone Pagamentos S/A (BACEN-regulated); Pagar.me's own modality `not found` [PGM-12] | LGPD cited for KYC results; residency `not found` [PGM-12] |
| **ASA** | PCI DSS Level 1 provider, but API/server-side tokenization leaves "sua infraestrutura no escopo"; only hosted pages reduce scope [ASA-67] | Instituição de pagamento code 461 [ASA-1] | LGPD statement `not found`; residency `not found` |
| **IUG** | PCI DSS certified; iugu.js keeps PAN off the platform; server-side `/v1/payment_token` only for PCI-certified companies [IUG-4][IUG-31][IUG-32] | Instituição de pagamento since 2020 (e-money issuer, prepaid account) [IUG-3][IUG-4] | LGPD in security page and contract; residency `not found` [IUG-59] |
| **PGB** | PCI certified; non-PCI integrators must use `card.encrypted` [PGB-12-pci][PGB-10] | Instituição de pagamento (acquirer, e-money issuer, account manager, instrument issuer); authorisation number `not found` [PGB-10] | LGPD referenced in the contract; residency `not found` [PGB-10] |
| **STR** | PCI Service Provider Level 1; Elements keep card data off the platform; SOC 1/2 [STR-19] | Instituição de pagamento, modalidade credenciadora [STR-1] | Privacy center does not name LGPD; data may be stored in any country where Stripe operates [STR-42] |
| **ADY** | Vault and Sessions flow qualify for SAQ A [ADY-10] | Instituição de pagamento authorised 23 Nov 2023 (secondary) [ADY-17] | `not found` |

**Observations**

- All seven are Banco Central-authorised payment institutions, so the sub-account money sits in regulated accounts everywhere; the difference is whose name is on the account (the store's at Mercado Pago, PagBank, Asaas, Iugu, Pagar.me; the platform's balance at Stripe and Adyen until transfer).
- Asaas is the only one whose documented integration leaves the platform in PCI scope for card data; the others offer a browser tokenization path.

## 9. International readiness

| | Countries and currencies | Cross-border and same-API reuse |
|---|---|---|
| **MP** | Split 1:1 in AR, BR, CL, CO, MX, PE, UY; one developer portal per country; BRL only in Brazil; accounts restricted to residents [MP-12][MP-11] | No multi-currency per account (`not found`); a second market means a second country account and portal [MP-26] |
| **PGM** | Brazil and BRL only [PGM-25] | `not found` |
| **ASA** | Brazil and BRL only [ASA-1] | Not offered |
| **IUG** | Brazil and BRL only [IUG-13] | `not found` |
| **PGB** | Domestic API BRL only; separate "PagSeguro International" unit (22 countries) with unknown API overlap [PGB-12-objeto-order][PGB-15] | `not found` |
| **STR** | 135+ presentment currencies; BR account settles in BRL; same API everywhere [STR-24][STR-2] | Cross-border payouts only for platforms in US, UK, EEA, CA, CH; a BR platform pays BR accounts only; other countries need regional platform accounts [STR-10][STR-5] |
| **ADY** | Single global platform, multi-currency acquiring; Platforms onboarding EU/UK, CA, US, AU, HK, NZ, SG (Brazil unresolved) [ADY-1][ADY-3][ADY-4] | Same Checkout API and SDKs across countries; a balance platform and merchant account per region [ADY-20][ADY-26] |

**Observations**

- No provider gives a Brazilian platform cross-border payouts self-serve. Future markets will mean either a regional account with the same provider (Stripe, Adyen, Mercado Pago's other country sites) or a second provider, which the requirements already anticipate in the two-gateway data model.
- For the MVP this dimension should not drive the choice; it only favours keeping the provider behind the domain interface, as section 25 requires.

---

## 10. Fit against the requirements and shortlist

The matrix scores each provider against the hard requirements stated in the scope. **Yes** means documented on an official page; **Partial** means possible with a workaround or a commercial condition; **No** means documented as unavailable; `not found` means the official pages are silent. The references are in the sections above.

| Requirement (requirements.md section) | MP | PGM | ASA | IUG | PGB | STR | ADY |
|---|---|---|---|---|---|---|---|
| Checkout inside the platform (§4) | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| PIX (§5) | Yes | Yes | Yes | Yes | Yes | Partial: invite only | Partial: on request, local entity |
| Credit card with installments (§5) | Yes | Yes (21x) | Yes (21x) | Yes (12x) | Yes | `not found` | Yes (merchant computes interest) |
| Brazilian card brands beyond Visa/Mastercard | Yes | Yes | Yes | Yes | Yes | No | `not found` |
| Sub-account per store created through the API (§11) | No: OAuth link to the seller's own account | Yes, Flex plan only | Yes | Yes | Yes, plus an app step by the store | Yes | Unresolved for Brazil |
| KYC by the gateway with status events (§11, §20) | Partial: no events | Yes | Yes | Partial: platform carries the analysis | Partial: finished in the PagBank app, no events | Yes | `not found` |
| Payout held until delivery and released by API (§11) | No | Partial: disable auto-payout, manual transfers | Yes: Conta Escrow (fee on request) | Partial: withdrawal block or master-held funds | Yes: custody up to 365 days | Partial: platform balance, 90-day limit, no manual payouts in Brazil | Partial: sweeps |
| Multi-store cart paid once, split to several stores (§10) | No: 1:1 only | Yes | Yes | Yes | No: single-seller cart | Yes | Yes |
| Card tokenization keeping the platform at SAQ A (§11) | Yes | Yes | No: server-side, "in scope" | Yes | Yes | Yes | Yes |
| Signed webhooks (§25) | Yes | `not found` | Token header only | Token header only | Yes | Yes | Yes |
| No fixed cost before revenue (§26) | Yes | `not found`: Flex is negotiated | Yes, except escrow and sub-account fees on request | No: subscription plan, R$ 1 per sub-account | Yes, rates negotiated | Partial: R$ 6 per active account | No: minimum invoice |
| Official Go SDK (§24) | Yes | Beta | No | No | No | Yes | Yes |
| Card pre-authorization of at least 7 days for a vehicle reservation (§29) | No: 5 days | Partial: MCC-dependent | Partial: 3 days, up to 25 for eligible accounts | No: 7 days then auto-cancel | Yes: 29 days | Partial: 7 days, 30 with extended authorization | Yes: 28 days default |
| Same provider for other countries later (§5) | Partial: per-country accounts | No | No | No | No | Yes, regional accounts | Yes, regional accounts |

**Reading of the matrix**

- Stripe and Adyen are the strongest engineering products but do not fit the Brazilian MVP as published: Stripe has PIX by invitation, no documented installments and no Elo, Hipercard or Amex; Adyen requires a minimum invoice and its platform product is not listed for Brazil. Both stay relevant for future markets.
- Mercado Pago and Iugu fail requirements that the design treats as foundations: Mercado Pago gives the platform no control over sub-accounts or release; Iugu is a subscription product and shifts KYC liability to the platform.
- Three providers satisfy the core marketplace model on paper: **Pagar.me** (complete recipient model with liability flags, biometric KYC with webhooks, network tokens, Go SDK in beta; open point: split is sold only in the negotiated Flex plan, and no escrow feature is named), **Asaas** (named escrow for delivery confirmation, unlimited split recipients, published flat fees, no monthly cost; open points: server-side card tokenization keeps the platform in PCI scope, no Go SDK, token-only webhooks, escrow and sub-account fees on request, regulatory ramp of 10 sub-accounts for 60 days) and **PagBank** (native custody up to 365 days with manual release, liable and chargeback-transfer flags, encrypted card capture, longest pre-authorization; open points: split only for single-seller carts, which contradicts section 10, store must finish verification in the PagBank app, negotiated rates, no server SDK).

**Suggested shortlist and next step**

1. Pagar.me and Asaas as the two candidates to engage commercially, with PagBank as the third if its single-seller-cart restriction can be lifted or worked around with one charge per store.
2. Questions to put to each of them, because the public pages do not answer them: fixed costs of the marketplace plan (Pagar.me Flex), escrow and sub-account fees (Asaas), chargeback fee, anticipation rate, maximum transaction amount for vehicles, webhook signature (Pagar.me), browser-side tokenization availability (Asaas), and the timeline of homologation or regulatory ramps.
3. A sandbox trial of the split, KYC and hold flows at the two finalists before the decision, noting that Pagar.me's PIX simulator does not work with split and Asaas approves accounts automatically in sandbox, so some flows can only be verified in production with small amounts.

## 11. Implications for the requirements

Each item names the section of `docs/requirements.md` it affects and whether the analysis suggests **changing** a decision, **confirming** a proposal or open topic, or **adding** something the document does not cover yet. Nothing here is decided; the owner decides.

### 11.1 Payments (§11) and the gateway open topic (§29)

1. **Confirm (§11, payout hold):** every candidate can hold a store's share until an event, but through different mechanisms: a named escrow (Asaas, PagBank), disabled automatic payouts plus manual transfers (Pagar.me), or platform-held balance (Stripe, with a 90-day limit). Suggestion: keep the proposal and add that the order model records who holds the money (store sub-account under escrow, or platform balance) and the release event that ends the hold, so the hold survives a gateway change.
2. **Add (§11):** payout release has provider limits: 365 days maximum custody at PagBank, 90 days at Stripe, 3 to 25 days of pre-authorization at Asaas. The platform's rule "held until delivery confirmed" needs a fallback deadline (automatic release or dispute) that fits within the provider's maximum.
3. **Confirm and detail (§11, KYC through the gateway):** the gateway runs identity verification at Pagar.me, Asaas and Stripe and reports it through webhooks; PagBank requires the store to finish in its own app; Iugu contractually leaves the analysis to the platform. Suggestion: model store onboarding as a state machine (created, documents pending, under review at provider, active, refused, suspended) fed by provider events, and make "the gateway performs KYC" a selection criterion rather than an assumption.
4. **Add (§11, §18):** KYC results come back as status only (approved, refused, redo), explicitly for LGPD reasons at Pagar.me. The platform should store the status and the provider's reference, never the documents, and prefer hosted or link-based document collection (Pagar.me KYC link, Asaas onboarding URL, Stripe Account Links) to API-only onboarding, which puts sensitive documents on the platform.
5. **Confirm and sharpen (§11, saved cards):** browser-side tokenization that keeps the platform at PCI SAQ A exists at six of seven providers; Asaas documents server-side tokenization that keeps "your infrastructure in scope". Suggestion: write "the card number never transits the platform's servers" as the requirement, not only "never stored", and use it as a selection criterion.
6. **Add (§29, installments):** who computes buyer-paid interest differs: PagBank's Fees API and Iugu compute it; Asaas, Adyen and Pagar.me expect the platform to send installment values. The seller-funded cost of interest-free installments is expressed as a monthly rate (PagBank 2,99% a.m.) or as tiers (Asaas 3,49% / 3,99% / 4,29%). Suggestion: the platform owns the installment plan (number of installments, interest-free limit per store or listing, resulting amounts) and treats the provider's cost table as configuration, so the checkout shows the same plans whatever the gateway.
7. **Add (§11, §29 vehicles):** a card pre-authorization is a poor fit for a vehicle reservation held for days: windows are 5 days (Mercado Pago), 7 days (Iugu, Stripe standard), 3 to 25 days (Asaas eligible accounts), 29 days (PagBank), 28 days (Adyen). Suggestion: model the reservation as a captured payment (PIX or card) held in escrow or platform balance, with pre-authorization as an optimisation only where the window allows.
8. **Add (§29 vehicles, §11):** maximum transaction amounts are either unpublished (Asaas, Iugu, Mercado Pago) or set by internal policy (PagBank "conforme políticas internas"); Stripe caps PIX at USD 3.000 equivalent. A vehicle price or even a deposit may exceed provider limits; add "maximum amount per transaction and per method" to the questions for the commercial proposal.
9. **Add (§11, §10):** PagBank's split works only for single-seller carts. The requirement "a cart with items from several stores ... paid with a single payment" therefore excludes PagBank unless the platform is the sole receiver and redistributes, which its public transfer API does not document. Record multi-recipient split per payment as a selection criterion.
10. **Add (§11, §4):** the platform's own stores also need a recipient or sub-account at the gateway, and the platform's commission lands in the platform's account; some providers (Mercado Pago 1:1) put the platform fee on the same payment while others (Asaas remainder, Pagar.me remainder rules, Stripe separate charges) let the platform keep the remainder. The order model should record the commission as a split line, not derive it after the fact.

### 11.2 External integrations (§25) and testing (§27)

11. **Add (§25, webhooks):** verification differs by provider: HMAC signatures at Mercado Pago, Stripe, Adyen and PagBank (new scheme); a shared token header at Asaas and Iugu; nothing documented at Pagar.me. Acknowledgement deadlines are short (22 seconds at Mercado Pago, 10 seconds at Asaas). Suggestion: the webhook endpoint verifies the signature where it exists, always re-reads the object from the provider before acting, stores the event id for idempotency, and answers immediately while processing asynchronously.
12. **Add (§27, §25):** sandbox coverage is incomplete for the flows that matter most: Iugu cannot create sub-accounts or transfer in sandbox; Pagar.me's PIX simulator does not work with split; Stripe cannot link a platform sandbox to connected-account sandboxes; Asaas approves every account automatically. The rule "tests use fake implementations" stays, but end-to-end validation of split and payout needs a documented manual test plan against real accounts with small amounts.
13. **Add (§25, §24):** official Go SDKs exist at Stripe, Adyen, Mercado Pago and Pagar.me (beta) only. For Asaas, Iugu or PagBank the adapter is written against their embedded OpenAPI definitions. Not a blocker under the ports-and-adapters rule, but it is extra work to estimate.
14. **Confirm (§25, provider-issued identifiers):** every provider uses its own identifiers for accounts, charges, splits and transfers (Pagar.me is even migrating recipient ids from `re_` to `pp_`). Storing every external identifier with the provider that issued it, as already required, is the right call.

### 11.3 Cost (§26) and business model (§4)

15. **Add (§26):** fixed costs to list in the cost inventory: Iugu subscription plus R$ 1 per sub-account per month and R$ 2,50 per withdrawal; Stripe R$ 6 per active connected account per month plus 0,25% + R$ 0,67 per payout; Asaas escrow fee per enabled sub-account (on request); Adyen minimum invoice; Stripe dispute fee R$ 55. The scale-to-zero premise rules out per-sub-account monthly fees while the marketplace has many small or dormant stores.
16. **Add (§4, §14):** processing fees are not returned on refunds at any provider (Asaas, Pagar.me, Stripe, Iugu even re-charges MDR). Decide who absorbs the non-refunded fee on a cancellation or return: the store, the platform, or the buyer through a deduction, and reflect it in the commission-refund rule already raised by the competitive analysis.
17. **Add (§4):** a flat PIX fee (Asaas R$ 1,99) versus a percentage (0,99% to 1,89% elsewhere) changes the economics of high-ticket vehicle deposits and of low-ticket electronics accessories in opposite directions. When comparing proposals, evaluate the fee on the expected ticket distribution of each marketplace, not on a single average.

### 11.4 Disputes and trust (§14, §20)

18. **Add (§14):** chargeback evidence is submitted through the provider's API under per-case deadlines (10 calendar days at Mercado Pago; per-case `due_date` at PagBank; 7 to 21 days at Stripe), often as a single PDF with page limits (PagBank 18 pages, Pagar.me 10 pages, Iugu 10 pages). Suggestion: the order keeps an evidence pack (tracking events, delivery proof, invoice, buyer messages) that can be exported as one PDF, and the dispute deadline becomes a notification event.
19. **Add (§20):** providers impose their own ramps on new stores: recipients cannot withdraw until active (Pagar.me), a 60-day evaluation period with 10 sub-accounts and R$ 2.000 each (Asaas), holds of up to 15 days for new sellers (Pagar.me). The platform's "lower limits for new stores" should be designed on top of, not instead of, the provider's limits, and the launch plan must schedule the provider's homologation.
20. **Add (§20, §11):** all contracts allow the provider to block balances or impose reserves unilaterally (Iugu, PagBank, Mercado Pago). Store-facing texts about payout timing must say that the payment institution can hold funds, and the platform's payout screen should reflect provider-side blocks received by webhook (Asaas `BALANCE_VALUE_BLOCKED`).

### 11.5 Markets (§5)

21. **Confirm (§5, §29 cross-border):** no provider offers a Brazilian platform cross-border payouts self-serve; Stripe and Adyen require regional platform accounts, Mercado Pago has one account per country, the others are Brazil-only. The two-gateway data model in §11 is therefore not only a migration safeguard but the likely shape of the first foreign market.
22. **Confirm (§5):** PIX and credit card cover the MVP; boleto is cheap at Asaas (R$ 1,99) and available everywhere, but adds settlement delay and reconciliation, so leaving it out of the MVP holds.

### 11.6 Gaps in this analysis

- Most Brazilian providers publish only headline rates; marketplace pricing, chargeback fees, anticipation rates, sub-account fees and transaction limits are "on request" and must come from commercial proposals.
- Adyen for Platforms availability in Brazil is contradictory across Adyen's own pages and must be confirmed with Adyen.
- Help centers of Asaas and Iugu blocked automated reading; the affected facts are marked as search snippets.

---

## Sources

All sources were accessed on 2026-09-17. Entries marked *(secondary)* or *(search snippet only)* are not official pages of the provider. Where a page shows its own update date, it is noted.

### Mercado Pago (MP)

- **[MP-1]** https://www.mercadopago.com.br/ajuda/33399 — Checkout fee table by method and release term
- **[MP-3]** https://www.mercadopago.com.br/ajuda/3605 — QR Code fees: Pix 0,49%, debit 1,99%, credit 4,98%/3,98%
- **[MP-5]** https://www.mercadopago.com.br/ajuda/30424 — "receber na hora" eligibility; new sellers 7/14/30 days; 7-day term since 17/06/2024
- **[MP-6]** https://www.mercadopago.com.br/ajuda/16229 — transfer fees: Pix/TED free
- **[MP-7]** https://www.mercadopago.com.br/ajuda/taxas-conta-digital_21766 — no monthly fee; withdrawal R$ 5,90; Pix transfer free
- **[MP-8]** https://www.mercadopago.com.br/ajuda/taxa-de-antecipacao_18727 — anticipation fee shown before each anticipation
- **[MP-10]** https://www.mercadopago.com.br/developers/pt/support/oferecer-parcelas-sem-acrescimo-para-compradores_454 — "parcelado vendedor" setup and taxa de parcelamento
- **[MP-11]** https://www.mercadopago.com.br/ajuda/termos-e-condicoes_300 — Terms (updated 7 Aug 2026): entity, BACEN modalities, residency/CPF-CNPJ, KYC, retention clauses
- **[MP-12]** https://www.mercadopago.com.br/developers/pt/docs/split-payments/split-1-1/overview — Split 1:1 overview, countries
- **[MP-13]** https://www.mercadopago.com.br/developers/pt/docs/split-payments/split-1-1/prerequisites — KYC 6, OAuth, 1:N restriction, release-date via commercial exec
- **[MP-14]** https://www.mercadopago.com.br/developers/pt/docs/split-payments/split-1-1/integration-configuration/create-configuration — OAuth flow, code 10 min, token 6 months
- **[MP-15]** https://www.mercadopago.com.br/developers/pt/docs/split-payments/split-1-1/integration-configuration/integrate-marketplace — marketplace_fee / application_fee, fee order, proportional refunds
- **[MP-16]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/reserve-capture-cancel — capture_mode manual, 5-day capture window, full capture only
- **[MP-17]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-integration/pix — Pix expiry 24 h default, 30 min–30 days, QR fields, 429
- **[MP-18]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-integration/cards — card token single-use, 7 days; brands via payment_methods
- **[MP-19]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/improve-payment-approval/saved-cards — Customers/Cards API
- **[MP-20]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/refunds-cancellations — 180-day refund window, full/partial
- **[MP-21]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/chargebacks/introduction — chargeback overview, fraud prevention
- **[MP-22]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/chargebacks/management — /v1/chargebacks endpoints, 10 files/10 MB, statuses
- **[MP-23]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/chargebacks/notifications — chargebacks topic payload
- **[MP-24]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/integrate-3ds — 3DS 2.0 config and liability shift
- **[MP-25]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/resources/test-cards — test card scenarios
- **[MP-26]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/resources/test-accounts — test accounts
- **[MP-27]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/notifications — Orders webhooks, x-signature, 22 s, 15 min
- **[MP-28]** https://www.mercadopago.com.br/developers/pt/docs/checkout-bricks/additional-content/your-integrations/notifications/webhooks — topic catalogue, HMAC, retries
- **[MP-29]** https://www.mercadopago.com.br/developers/pt/docs/checkout-bricks/additional-content/security/pci — PCI DSS, SAQ A vs D
- **[MP-30]** https://www.mercadopago.com.br/developers/pt/docs/checkout-bricks/card-payment-brick/advanced-features/configure-installments — min/maxInstallments
- **[MP-31]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/integration-model — Orders vs Payments API, processing_mode, capture_mode
- **[MP-33]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/payment-management/integration-errors — 429 usage_quota_exceeded, Retry-After
- **[MP-34]** https://www.mercadopago.com.br/developers/pt/docs/checkout-api-orders/resources/reports/released-money/introduction — Releases report, reserve-/Payout/Dispute, API generation
- **[MP-37]** https://www.mercadopago.com.br/developers/pt/docs/sdks-library/landing — 11 official SDKs incl. Go
- **[MP-38]** https://github.com/mercadopago/sdk-go — official Go SDK, Go 1.23+
- **[MP-39]** https://status.mercadopago.com/ — status page, 99.95% 90-day uptime
- **[MP-40]** https://www.mercadopago.com.br/ajuda/programa-protecao-vendedor_527 — Seller Protection Program terms (updated 09/09/2025)
- **[MP-41]** https://www.mercadopago.com.br/ajuda/23881 — chargeback: 10 dias corridos to prove delivery, up to 120 days

### Pagar.me (PGM)

- **[PGM-2]** https://docs.pagar.me/docs — docs intro: gateway + PSP, tokenizecard.js, checkout, versions
- **[PGM-3]** https://www.pagar.me/ofertas — Essencial/Flex pricing (0,99% PIX, 4,19%, 13,63% 6x, 21x, 1 day, 15-day retention footnote, split only in Flex)
- **[PGM-5]** https://docs.pagar.me/docs/recebedores-2 (updated 2026-04-06) — recipient params, automatic withdrawals rules, 60-day inactivity, bank account change rule
- **[PGM-6]** https://docs.pagar.me/reference/split-1 (updated 2026-04-06) — split object, options, PSP-only, responsibility rule
- **[PGM-7]** https://docs.pagar.me/reference/recebedores-1 (updated 2026-04-06) — recipient object, status definitions
- **[PGM-8]** https://docs.pagar.me/reference/criar-recebedor-1 — create recipient fields, transfer/anticipation settings
- **[PGM-9]** https://docs.pagar.me/page/novas-regras-para-criação-de-sellers-de-marketplace-c-v5 (updated 2026-05-21) — register_information required fields PF/PJ, managing partner rule, dates
- **[PGM-11]** https://docs.pagar.me/reference/criar-link-recebedor (updated 2026-04-06) — POST /recipients/{id}/kyc_link
- **[PGM-12]** https://docs.pagar.me/page/adequação-de-marketplace-para-mudanças-regulatórias (updated 2026-04-06) — KYC flow, 20 min QR, 3 attempts, 24 h, Conta Digital/Stone Pagamentos, Res. 264, FAQ (foreigners, refused recipients)
- **[PGM-13]** https://docs.pagar.me/page/guia-rápido-sobre-antecipações (updated 2026-04-06) — 30/31/61/91-day schedule, fee as monthly simple discount
- **[PGM-14]** https://docs.pagar.me/reference/obtendo-os-limites-de-antecipação (updated 2026-04-06) — limits endpoint, fee fields, OpenAPI 3.1.0
- **[PGM-16]** https://docs.pagar.me/reference/eventos-de-webhook-1 — event catalogue incl. charge.chargedback deprecation 30/09/2026, antifraud events, recipient events
- **[PGM-17]** https://docs.pagar.me/reference/criando-uma-transferência (updated 2026-04-06) — POST /transfers, Idempotency-Key, example fee 367, type ted
- **[PGM-18]** https://docs.pagar.me/reference/pix-2 — PIX object fields, statuses, refund via cancel
- **[PGM-19]** https://docs.pagar.me/docs/simulador-pix — PIX sandbox rules (R$500 threshold, no split)
- **[PGM-20]** https://docs.pagar.me/reference/cartão-de-crédito-1 — credit_card object, operation_type, 3DS 2.1/2.2, statement descriptor 13/22, pre-auth needs acquirer
- **[PGM-21]** https://pagarme.helpjuice.com/pt_BR/p2-manual-da-dashboard/taxas-como-vejo-as-minhas-taxas — fee catalogue wording (help center)
- **[PGM-22]** https://pagarme.helpjuice.com/pt_BR/estorno-de-vendas-como-funciona-prazos-e-principais-duvidas — refund windows 180/90 days, fee refund rules, D+2 negative agenda (search snippet for D+2)
- **[PGM-23]** https://pagarme.helpjuice.com/pt_BR/p2-funcionalidades/13marketplace-quem-arca-com-as-taxas-em-uma-regra-de-split — who pays fees in split
- **[PGM-24]** https://pagarme.helpjuice.com/pt_BR/marketplace-saldo-global-do-marketplace — global balance, negative recipients example
- **[PGM-25]** https://docs.pagar.me/reference/criar-pedido-2 — create order, brands, currency BRL
- **[PGM-26]** https://pagarme.helpjuice.com/pt_BR/p1-funcionalidades/marketplace-como-funciona-o-split-de-pagamentos — payments to seller's own bank account, dashboard only for marketplace
- **[PGM-28]** https://docs.pagar.me/docs/autenticação-via-3ds — 3DS product, Visa/Mastercard, liability shift
- **[PGM-31]** https://docs.pagar.me/docs/token-de-bandeira (updated 2026-04-06) — network tokenization (Visa/Mastercard), pass-through
- **[PGM-32]** https://docs.pagar.me/docs/wallets (updated 2026-04-06) — card wallet, Zero Dollar Auth
- **[PGM-33]** https://docs.pagar.me/docs/cobrança — charge statuses (authorized_pending_capture, partial_capture, partial_refunded...)
- **[PGM-34]** https://docs.pagar.me/docs/visa-novo-prazo-para-captura-de-transações (updated 2026-04-06) — capture windows 5 h / 4 days / 29 days vehicles
- **[PGM-38]** https://pagarme.helpjuice.com/pt_BR/p1-saques-recebimentos-e-antecipação/antecipação-como-funcionam-os-modelos-de-antecipação — anticipation models, daily approval
- **[PGM-39]** https://pagarme.helpjuice.com/pt_BR/p1-meios-de-pagamento/pix-saiba-mais-sobre-esse-meio-de-pagamento — PIX immediate balance, 90-day refund, 10-attempt limit; plus https://docs.pagar.me/docs/pix-1 (SPI, partial refunds, split)
- **[PGM-44]** https://docs.pagar.me/page/chargeback-novo-status-na-cobrança (updated 2026-07-16) — chargedback status rules
- **[PGM-45]** https://docs.pagar.me/reference/get_v1-disputes (updated 2026-05-14) — Disputes API, Stone base URLs, cycle statuses
- **[PGM-47]** https://docs.pagar.me/reference/rate-limit — same as S43 (PIX cancel limit)
- **[PGM-48]** https://docs.pagar.me/reference/autenticação-2 (updated 2026-04-06) — Basic Auth, key prefixes, single endpoint
- **[PGM-49]** https://docs.pagar.me/docs/webhooks (updated 2026-04-06) — webhook basics, configurable retries, event list
- **[PGM-51]** https://docs.pagar.me/docs/simulador-de-cartão-de-crédito (updated 2026-04-06) — test card numbers
- **[PGM-52]** https://docs.pagar.me/page/ambiente-de-teste-para-prova-de-vida — KYC sandbox (index entry)
- **[PGM-53]** https://docs.pagar.me/docs/bibliotecas-1 (updated 2026-08-06) — SDK list incl. Golang (beta), v7 for 28/08/2026 changes
- **[PGM-54]** https://status.pagar.me/ — status page exists ("Pagarme Status")

### Asaas (ASA)

- **[ASA-1]** https://www.asaas.com/precos-e-taxas — all fees, promo, transfers, BACEN code 461, scale, FAQ (D+32, 1,25%/1,70%). (www.asaas.com/precos redirected to login.)
- **[ASA-8]** https://docs.asaas.com/docs/criacao-de-subcontas.md — sub-account creation, CNPJ parent, evaluation limits, fees "consulte Taxas", sandbox 20/day
- **[ASA-9]** https://docs.asaas.com/docs/criação-de-subcontas-baas.md — BaaS model, Resoluções 16/17, webhooks at creation
- **[ASA-10]** https://docs.asaas.com/docs/split-de-pagamentos.md — split rules, netValue, statuses, divergence block
- **[ASA-11]** https://docs.asaas.com/docs/faq-do-split.md — no recipient limit, no scheduled split, refunds reverse splits
- **[ASA-12]** https://docs.asaas.com/docs/introducao-conta-escrow.md — escrow concept, marketplace use case, fee
- **[ASA-13]** https://docs.asaas.com/docs/habilitando-a-conta-escrow-para-as-subcontas.md — endpoints, isFeePayer, daysToExpire
- **[ASA-15]** https://docs.asaas.com/docs/desbloqueio-dos-valores.md — manual release POST /v3/escrow/{id}/finish
- **[ASA-17]** https://docs.asaas.com/docs/criar-cobrança-com-3ds.md — 3DS contract, enablement by support
- **[ASA-18]** https://docs.asaas.com/docs/tokenization.md — tokenization permission error, account-manager enablement
- **[ASA-20]** https://docs.asaas.com/reference/criar-subconta.md — OpenAPI required fields, companyType enum, webhooks
- **[ASA-21]** https://docs.asaas.com/docs/detalhamento-do-fluxo-de-aprovação-de-subcontas.md — non-BaaS vs BaaS approval, documents, 15 s wait
- **[ASA-22]** https://docs.asaas.com/docs/onboarding-e-envio-de-documentos-via-link.md — onboardingUrl, 48 h analysis, 5-min liveness
- **[ASA-23]** https://docs.asaas.com/docs/webhook-para-verificar-situacao-da-conta.md — ACCOUNT_STATUS_* events
- **[ASA-24]** https://docs.asaas.com/reference/estornar-cobranca.md — refund rules, fees not returned, 10 business days, splitRefunds
- **[ASA-25]** https://docs.asaas.com/docs/estornos.md — refunds array, refundedSplits
- **[ASA-26]** https://docs.asaas.com/docs/chargeback.md — chargeback status/reason enums
- **[ASA-27]** https://docs.asaas.com/docs/faq-periodo-de-avaliacao.md — evaluation-period limits and blocks
- **[ASA-28]** https://docs.asaas.com/docs/transferencia-para-contas-de-outra-instituicao-pix-ted.md — external transfers, PIX/TED, scheduleDate
- **[ASA-29]** https://docs.asaas.com/docs/transferencia-para-conta-asaas.md and https://docs.asaas.com/docs/faq-de-transferências.md — internal transfers, linked accounts only, cancel
- **[ASA-34]** https://docs.asaas.com/docs/cobrancas-via-pix.md — PIX QR, 12-month expiry
- **[ASA-35]** https://docs.asaas.com/docs/cobrancas-via-cartao-de-credito.md — API card flow, debit via invoiceUrl, token reuse, 21x/12x
- **[ASA-36]** https://docs.asaas.com/reference/criar-cobranca-com-cartao-de-credito.md — authorizeOnly, 3–25 days, HTTPS, brands enum
- **[ASA-37]** https://docs.asaas.com/reference/capturar-cobranca-com-pre-autorizacao.md — capture endpoint, AUTHORIZED status
- **[ASA-40]** https://docs.asaas.com/docs/criar-uma-cobranca-parcelada.md — installment fields, rounding
- **[ASA-43]** https://blog.asaas.com/compensacao-de-pagamento/ — D+32, D+64, debit D+3, boleto 13h30 (secondary)
- **[ASA-44]** https://docs.asaas.com/docs/antecipacoes.md — anticipation API and events
- **[ASA-45b]** https://docs.asaas.com/docs/webhook-para-cobrancas.md — 31 PAYMENT_* events, refund/chargeback flows
- **[ASA-46]** https://docs.asaas.com/docs/mecanismo-para-validacao-de-saque-via-webhooks.md — withdrawal validation webhook
- **[ASA-47]** https://docs.asaas.com/docs/eventos-para-bloqueios-de-saldo.md — BALANCE_VALUE_* events
- **[ASA-50]** https://docs.asaas.com/docs/o-que-pode-ser-testado.md — sandbox coverage, chargeback test by e-mail
- **[ASA-52]** https://docs.asaas.com/docs/autenticação-1.md — auth headers, base URLs, TLS
- **[ASA-53]** https://docs.asaas.com/docs/sobre-os-webhooks.md — 10 webhooks, at-least-once, 14 days, token header
- **[ASA-54]** https://docs.asaas.com/docs/criar-novo-webhook-pela-api.md — POST /v3/webhooks fields, token 32–255
- **[ASA-55]** https://docs.asaas.com/docs/faq-de-webhooks.md — only HTTP 200, 10 s timeout, no Bearer
- **[ASA-58]** https://docs.asaas.com/docs/sandbox.md — sandbox behaviour
- **[ASA-59]** https://docs.asaas.com/docs/testando-pagamento-com-cartão-de-crédito.md — test cards
- **[ASA-60]** https://docs.asaas.com/docs/testar-pagamento-de-qrcodes-pix.md — PIX sandbox payment simulation
- **[ASA-61]** https://docs.asaas.com/docs/sdks.md — Java-only official SDK
- **[ASA-63]** https://docs.asaas.com/docs/postman.md — Postman collection
- **[ASA-65]** https://docs.asaas.com/changelog.md — changelog index
- **[ASA-66]** https://docs.asaas.com/reference/rate-e-quota-limit.md — 25.000/12 h, 50 concurrent GET
- **[ASA-67]** https://docs.asaas.com/docs/pci-dss-1.md — PCI DSS Level 1, scope table

### Iugu (IUG)

- **[IUG-1]** https://www.iugu.com/planos — plan FAQ (split not in Essencial; monthly subscription model; 30-day cancellation; PJ required), footer legal entity/CNPJ
- **[IUG-2]** https://www.iugu.com/saiba-mais — published fee table for Essencial, Essencial + Split, Motor iugu (PIX 0,99%, boleto R$ 2,19, card 3,34%/4,28%/4,79%, R$ 0,40 processing, R$ 1,00 sub-account maintenance, R$ 2,50 sub-account withdrawal)
- **[IUG-3]** https://www.iugu.com/pagamentos-online — "IP autorizada desde 2020, Lei 12.865/2013", PCI-DSS, 3DS 2.0, antifraude claims
- **[IUG-4]** https://www.iugu.com/juridico/contrato — Terms of use: IP modality, settlement terms 3.4.1.1, sub-account mandate and joint liability, KYC duties, reserves/guarantees, blocking, MED 2%, clause 6.1.7 fee not refunded, PCI annual audit, anti-fraud by plan
- **[IUG-7]** https://herospark.com/blog/iugu/ — (secondary) fee table repeat and "plano mais barato R$ 149,00 mensais"
- **[IUG-9]** https://dev.iugu.com/docs/split-de-pagamentos (updated 2025-10-31) — split parameters, permit_aggregated, no 100% rule, unlimited recipients via API, per-invoice/subscription split
- **[IUG-10]** https://dev.iugu.com/docs/split-de-pagamento (updated 2025-10-31) — who pays fees, settlement of split, worked examples
- **[IUG-11]** https://dev.iugu.com/reference/criar-subconta — POST /v1/marketplace/create_account, production only, cannot delete, RSA headers, response tokens
- **[IUG-12]** https://dev.iugu.com/docs/criar-verificar-e-configurar-subconta (updated 2026-06-23) and https://dev.iugu.com/docs/configurar-subconta-por-api — sub-account lifecycle, 24h/2-business-day verification, deactivate rules, auto_withdraw/auto_advance config, bank domicile change
- **[IUG-13]** https://dev.iugu.com/reference/criar-fatura — invoice parameters (payable_with, expires_in, splits, max_installments_value, automatic_pix, order_id), currency BRL
- **[IUG-14]** https://dev.iugu.com/reference/enviar-verificação-de-subconta (updated 2026-06-10) — KYC fields, documents, 24h window, 2 business days, 401 for unverified, bank-holder rule
- **[IUG-15]** https://dev.iugu.com/docs/estorno (updated 2025-10-31) — refund rules per method (card 180 days/30-60 days; Pix 90 days full only; boleto none)
- **[IUG-16]** https://dev.iugu.com/docs/realizar-o-reembolso-de-faturas-estorno-por-api (updated 2026-09-01) — refund API, proportional split refunds, MDR charged on refunds, installment refund mechanics
- **[IUG-17]** https://dev.iugu.com/reference/criar-split-1 (updated 2025-10-31) — POST /v1/splits OpenAPI definition with all split fields
- **[IUG-18]** https://dev.iugu.com/docs/cobrança-em-duas-etapas (updated 2025-10-31) — pre-authorization/capture flow, 7-day auto-cancel, in_analysis status
- **[IUG-19]** https://dev.iugu.com/reference/capturar-fatura — POST /v1/invoices/{id}/capture
- **[IUG-20]** https://dev.iugu.com/reference/configurar-conta and https://dev.iugu.com/docs/cc-configuracoes-extras — account configuration (two_step_transaction, max_installments, max_installments_without_interest, disabled_withdraw, customer_minimum_balance_cents, auto_withdraw*, auto_advance*)
- **[IUG-21]** https://dev.iugu.com/docs/gatilhos-kyc (updated 2025-10-31) — KYC webhooks and charge_limit_cents
- **[IUG-23]** https://dev.iugu.com/docs/transferências (updated 2025-10-31) — withdrawal D+1, min R$ 5,00, no third-party withdrawals, transfers between iugu accounts min 1 cent
- **[IUG-24]** https://dev.iugu.com/docs/realizar-antecipação-de-parcelas-via-api (updated 2025-10-31) — anticipation API, compound monthly interest, 200-installment cap, 9-16h window
- **[IUG-26]** https://dev.iugu.com/reference/pedido-de-saque and https://dev.iugu.com/reference/transferir-valor (updated 2026-07-28) — withdrawal and transfer endpoints, RSA, production only
- **[IUG-28]** https://dev.iugu.com/docs/realizar-cobrança-com-pix-por-api (updated 2025-10-31) — PIX charge, qrcode/qrcode_text, invoice.released webhook, Pix enablement
- **[IUG-31]** https://dev.iugu.com/docs/tokenizacao-de-cartao-de-credito (updated 2025-10-31) — iugu.js usage, test mode, brand validation list, PCI warning for /v1/payment_token
- **[IUG-32]** https://dev.iugu.com/docs/tokens-de-cartão-de-crédito (updated 2025-10-31) — API vs iugu.js tokenization, PCI scope, token migration (GPG)
- **[IUG-33]** https://dev.iugu.com/reference/criar-metodo-de-pagamento — saved payment methods endpoint and customer_payment_method.new webhook
- **[IUG-34]** https://dev.iugu.com/docs/compartilhar-forma-de-pagamento (updated 2026-04-14) — shared payment methods across sub-accounts (must be created in master)
- **[IUG-35]** https://dev.iugu.com/reference/cobranca-direta — POST /v1/charge parameters (months 2-12, token single-use, master token usable in marketplace, order_id)
- **[IUG-43]** https://dev.iugu.com/reference/status-de-retorno-de-contestação (updated 2026-04-17) — chargeback statuses; invoice status chargeback webhook
- **[IUG-45]** https://dev.iugu.com/reference/disputar-contestacao (updated 2026-03-02) — dispute evidence limits (5 files, 10 pages, 8 MB)
- **[IUG-46]** https://dev.iugu.com/reference/introdução-a-api — base URL, test-mode 50 req/min and 1000 invoices/day limits
- **[IUG-47]** https://dev.iugu.com/reference/autenticação (updated 2026-07-28) and https://dev.iugu.com/docs/tokens-de-autenticacao (updated 2026-07-30) — auth methods, token types, RSA signing
- **[IUG-48]** https://dev.iugu.com/docs/chave-de-idempotência-1 (updated 2026-09-16) — Idempotency-Key semantics
- **[IUG-49]** https://dev.iugu.com/reference/criar-gatilho (updated 2025-10-31) — POST /v1/web_hooks, authorization field, 20-webhook limit
- **[IUG-53]** https://dev.iugu.com/reference/listar-eventos-disponíveis — event names (same as S42)
- **[IUG-55]** https://dev.iugu.com/docs/bibliotecas (updated 2025-10-31) — official SDK list (no Go)
- **[IUG-57]** https://dev.iugu.com/llms.txt — documentation index, changelog entries, multilingual intro pages
- **[IUG-58]** https://status.iugu.com/ — status page components
- **[IUG-59]** https://www.iugu.com/seguranca — PCI and LGPD statements

### PagBank (PGB)

- **[PGB-1]** https://pagbank.com.br/para-seu-negocio/online — online sales fee tables (débito 2,39%; crédito 14d 4,99%+R$0,40, 30d 3,99%+R$0,40; Pix 1,89%; 2,99%/month interest-free installments; free TED/PIX)
- **[PGB-2]** https://pagbank.com.br/para-seu-negocio/online/checkout — Checkout fees incl. boleto; FAQ "Recebimento em 14 dias: 4,99% + R$0,40 / 30 dias: 3,99% + R$0,40"
- **[PGB-3]** https://pagbank.com.br/para-seu-negocio/maquininhas/taxas-e-tarifas — card-present fee table by payout plan (na hora / 14 / 30 dias), 12x/18x footnote, "sem aluguel, mensalidade nem taxa de adesão"
- **[PGB-5]** https://faq.pagbank.com.br/duvida/quais-sao-as-taxas-para-vender-com-link-de-pagamento/1929 — Link campaign regulation with standard post-promo fee table (2x–12x), valid from 08/04/2026
- **[PGB-6]** https://faq.pagbank.com.br/duvida/taxas-para-vender-via-pix/1160 — Pix selling fee "limitada a 1,89%"
- **[PGB-7]** https://faq.pagbank.com.br/duvida/quais-sao-as-regras-de-cancelamento-de-uma-venda/2256 — refund types (total, parcial, proporcional, customizado), windows (API 350 days, Pix 90, Maestro/Banrisul 120), refund timing by method
- **[PGB-8]** https://pagbank.com.br/para-seu-negocio/vantagens-das-maquininhas/antecipacao-de-vendas — anticipation terms (rate on request, credit analysis)
- **[PGB-10]** https://acq-static-pages.pagseguro.com.br/website-cms-pages/Contrato_da_Conta_Pag_Bank_e_Outros_Servicos_994374b071.pdf (linked from https://pagbank.com.br/sobre/contrato-de-servicos) — service contract: legal entity, IP activities, Reserva Financeira (Cap. XV), clauses 7.2, 7.6, 8.2, 4.6.1 PCI, LGPD. No version date printed
- **[PGB-11]** https://pagbank.com.br/para-seu-negocio/online-integracao/split-de-pagamento — Split marketing/FAQ: single-seller cart, all parties need PagBank account, no product fee, anti-fraud included, fees negotiated
- **[PGB-12-3ds]** https://developer.pagbank.com.br/reference/criar-pagar-pedido-com-3ds-validacao-pagbank and https://developer.pagbank.com.br/reference/criar-sessao-autenticacao-3ds
- **[PGB-12-ambientes]** https://developer.pagbank.com.br/docs/ambientes-disponiveis
- **[PGB-12-cadastro]** https://developer.pagbank.com.br/docs/cadastro-de-clientes
- **[PGB-12-cancelamento-split]** https://developer.pagbank.com.br/reference/cancelamento-de-pedido-com-divisao-de-pagamento
- **[PGB-12-cancelar]** https://developer.pagbank.com.br/reference/cancelar-pagamento
- **[PGB-12-capturar]** https://developer.pagbank.com.br/reference/capturar-pagamento
- **[PGB-12-cartoes]** https://developer.pagbank.com.br/docs/cartoes-de-teste
- **[PGB-12-chargeback]** https://developer.pagbank.com.br/docs/chargeback (updated 2026-09-15)
- **[PGB-12-como-utilizar]** https://developer.pagbank.com.br/reference/como-utilizar-a-divisao-de-pagamento
- **[PGB-12-connect-auth]** https://developer.pagbank.com.br/docs/connect-authorization
- **[PGB-12-criar-conta]** https://developer.pagbank.com.br/reference/criar-conta
- **[PGB-12-criar-disputa]** https://developer.pagbank.com.br/reference/criar-disputa
- **[PGB-12-criar-pedido]** https://developer.pagbank.com.br/reference/criar-pedido
- **[PGB-12-custodia]** https://developer.pagbank.com.br/reference/crie-e-pague-um-pedido-com-custodia
- **[PGB-12-divisao]** https://developer.pagbank.com.br/reference/divisao-de-pagamento
- **[PGB-12-fees]** https://developer.pagbank.com.br/reference/consultar-taxas-transacao
- **[PGB-12-homologacao]** https://developer.pagbank.com.br/docs/solicitar-homologacao
- **[PGB-12-idempotencia]** https://developer.pagbank.com.br/docs/chaves-publicas-e-de-idempotencia
- **[PGB-12-liable]** https://developer.pagbank.com.br/reference/utilizar-o-mcc-vendedor-principal-liable
- **[PGB-12-liberar-custodia]** https://developer.pagbank.com.br/reference/liberar-divisao-de-pagamento-com-custodia
- **[PGB-12-llms]** https://developer.pagbank.com.br/llms.txt — docs index (no SDK page)
- **[PGB-12-objeto-order]** https://developer.pagbank.com.br/reference/objeto-order
- **[PGB-12-pci]** https://developer.pagbank.com.br/docs/seguranca-e-selo-pci
- **[PGB-12-pix-v2]** https://developer.pagbank.com.br/reference/criar-pedido-com-qr-code-pix-v2 (updated 2026-09-14)
- **[PGB-12-preferencia]** https://developer.pagbank.com.br/reference/criar-preferencia (updated 2026-09-15)
- **[PGB-12-recuperacao-chargeback]** https://developer.pagbank.com.br/reference/recuperacao-chargeback-de-secundario
- **[PGB-12-repasse]** https://developer.pagbank.com.br/reference/criar-transacao-com-repasse-de-taxa
- **[PGB-12-simulador]** https://developer.pagbank.com.br/docs/simulador
- **[PGB-12-status]** https://developer.pagbank.com.br/changelog/status-page (→ https://status.pagseguro.uol.com.br/)
- **[PGB-12-tokens-cards]** https://developer.pagbank.com.br/reference/validar-armanezar-cartao-pagbank
- **[PGB-12-validacao]** https://developer.pagbank.com.br/reference/validacao-de-autenticidade (updated 2026-09-15)
- **[PGB-12-webhooks]** https://developer.pagbank.com.br/reference/webhooks
- **[PGB-15]** https://international.pagseguro.com/ — PagSeguro International coverage claim "22 countries and 144 payment methods"

### Stripe (STR)

- **[STR-1]** https://stripe.com/br/legal/ssa — legal entity, CNPJ, "instituição de pagamento ... credenciadora", governing law, last modified 18 Nov 2025
- **[STR-2]** https://stripe.com/br/pricing — card 3,99% + R$ 0,39, +2% international, PIX 1,19% invite-only, Boleto R$ 3,45, disputes R$ 55/R$ 55, Smart Disputes 30%, refund policy, no monthly fees, Sigma R$ 50
- **[STR-3]** https://stripe.com/br/connect/pricing — R$ 6 per active account/month, 0,25% + R$ 0,67 per payout, "A partir de 3,99% + R$ 0,50", KYC included, 3DS "mediante tarifa adicional"
- **[STR-4]** https://docs.stripe.com/connect/charges — charge types, fee/refund/dispute liability per type, on_behalf_of
- **[STR-5]** https://docs.stripe.com/connect/separate-charges-and-transfers?platform=web&integration=custom&ui=elements&api-integration=paymentintents — BR in region list, transfer_group, source_transaction, same-region rule, reversals
- **[STR-8]** https://docs.stripe.com/connect/accounts — Standard/Express/Custom comparison; Express and Custom country lists (BR absent)
- **[STR-10]** https://docs.stripe.com/connect/cross-border-payouts — platform regions US/UK/EEA/CA/CH only
- **[STR-11]** https://docs.stripe.com/payments/pix — BR invite only, one-time, limits, refunds 90 days, Connect support, IOF; https://docs.stripe.com/payments/installments — index lists Mastercard/Mexico/Japan only
- **[STR-12]** https://docs.stripe.com/payments/setup-intents — saving cards without charge, usage
- **[STR-13]** https://docs.stripe.com/payouts — BR settlement 30/5 calendar days, 2 business days Pix/Boleto, "Brazil and India: Payouts are always automatic and daily", min payout 0.01 BRL, BR bank format
- **[STR-14]** https://docs.stripe.com/payments/place-a-hold-on-a-payment-method — 7-day CIT window, Visa MIT 5 days, partial capture, automatic delayed capture preview
- **[STR-15]** https://docs.stripe.com/refunds — fees not returned, 5–10 business days, Connect refunds, Boleto requires_action
- **[STR-16]** https://docs.stripe.com/disputes/how-disputes-work and https://support.stripe.com/questions/june-2025-pricing-updates-for-disputes — timelines, BRL 55/55, effective 17 Jun 2025
- **[STR-17]** https://stripe.com/br/radar/pricing — Radar tiers R$ 0,25/0,35/0,45; platform R$ 5/15/25 per active account
- **[STR-18]** https://docs.stripe.com/connect/payouts-bank-accounts — BR bank code + branch + account format
- **[STR-19]** https://docs.stripe.com/security — PCI Level 1, SAQ assistance, AES-256 vault, SOC
- **[STR-20]** https://docs.stripe.com/connect/required-verification-information (+ endpoints get-platform-countries, get-requirement-selections-for-platform-country?platformCountry=BR, get-requirements-for-setups for BR individual/company) — BR platform support, capabilities, CPF/CNPJ fields, br-2025 program
- **[STR-21]** https://docs.stripe.com/connect/handling-api-verification — requirements hash, account.updated, Files API, verification timing
- **[STR-23]** https://support.stripe.com/questions/accepted-payment-methods-in-brazil — Visa/Mastercard only, no Amex/Elo/Hipercard, no local debit, Google/Apple Pay, no installment info
- **[STR-24]** https://docs.stripe.com/currencies — 135+ currencies, min 0.50 BRL, max digits, FX-control note for Brazil
- **[STR-25]** https://docs.stripe.com/payments/pix/accept-a-payment?payment-ui=direct-api — QR fields, expires_after_seconds 10–259200 (default 14400), test CPF/emails
- **[STR-27]** https://support.stripe.com/questions/payouts-in-brazil-card-receivables — receivables registration statement; no anticipation info
- **[STR-28]** https://docs.stripe.com/payments/extended-authorization?platform=web&ui=elements — 30-day windows by brand/MCC, +0.08% Visa other MCCs, IC+ note
- **[STR-29]** https://docs.stripe.com/connect/manual-payouts — 90-day holding period, "doesn't provide escrow services"
- **[STR-30]** https://docs.stripe.com/connect/account-balances — holding funds patterns, reserves, negative balances, auto-debit countries
- **[STR-31]** https://docs.stripe.com/connect/instant-payouts — availability list (no BR), 1% fee
- **[STR-33]** https://docs.stripe.com/webhooks — signature, retries, ordering, Connect scope, 16 endpoints
- **[STR-34]** https://docs.stripe.com/payments/3d-secure — 3DS overview, optional outside SCA regions
- **[STR-35]** https://docs.stripe.com/api/idempotent_requests — Idempotency-Key rules
- **[STR-36]** https://docs.stripe.com/api/versioning — current version 2026-08-26.dahlia, Stripe-Version header
- **[STR-37]** https://docs.stripe.com/sandboxes — sandbox limits (Connect sandbox linking, IC+)
- **[STR-38]** https://docs.stripe.com/testing — test cards incl. BR Visa 4000000760000002, 3DS cards
- **[STR-39]** https://docs.stripe.com/sdks — server SDK list incl. Go, OpenAPI repo
- **[STR-40]** https://docs.stripe.com/rate-limits — 100/25 req/s, endpoint and Connect limits
- **[STR-41]** https://status.stripe.com/ — status page URL (content JS-rendered, not verifiable by fetch)
- **[STR-42]** https://stripe.com/br/connect — Connect marketing page for Brazil (no BRL prices; CNPJ footer)

### Adyen (ADY)

- **[ADY-1]** https://www.adyen.com/pricing — fixed fee $0.13, Interchange++ + 0.60%, PIX/boleto Brazil "volume-based", no setup/monthly fees, minimum invoice exists
- **[ADY-2]** https://www.adyen.com/pt_BR/precos — €0.11 fixed fee, PIX/boleto "contrato direto", "fatura mínima", meal vouchers
- **[ADY-3]** https://docs.adyen.com/platforms/ — Platforms overview; onboardable countries list (no Brazil); "As a platform" definition
- **[ADY-4]** https://docs.adyen.com/marketplaces/ — Marketplaces overview; country list (no Brazil); components
- **[ADY-5]** https://docs.adyen.com/payment-methods/pix/ (via r.jina.ai) — PIX overview: local entity required, refunds, recurring, daily transfer, 90-day bank dispute
- **[ADY-6]** https://docs.adyen.com/payment-methods/pix/api-only/ (via r.jina.ai) — PIX API fields, 24h/1h validity, sessionValidity, webhooks, 90-day refund, test simulation
- **[ADY-7]** https://docs.adyen.com/online-payments/adjust-authorisation/ — PreAuth flag, 28-day default, scheme maxima
- **[ADY-8]** https://docs.adyen.com/payment-methods/cards/credit-card-installments (via r.jina.ai) — Brazil installments <100, interest in amount.value, 30-day settlement delay
- **[ADY-9]** https://docs.adyen.com/marketplaces/onboard-users/ — hosted vs API-only onboarding, legal entity types, country list (no Brazil), LEM/Configuration APIs
- **[ADY-10]** https://docs.adyen.com/online-payments/tokenization/ — Vault, recurring types, SAQ A, network tokenization
- **[ADY-11]** https://docs.adyen.com/marketplaces/split-transactions/split-payments-at-authorization (via r.jina.ai) — split types and rules
- **[ADY-12]** https://docs.adyen.com/marketplaces/transaction-fees (via r.jina.ai) — fee booking defaults, AcquiringFees/AdyenFees/PaymentFee
- **[ADY-13]** https://docs.adyen.com/risk-management/understanding-disputes/dispute-timeframes (via r.jina.ai) — dispute windows per scheme
- **[ADY-14]** https://docs.adyen.com/online-payments/refund/ — refund endpoint, 40 business days, webhooks
- **[ADY-17]** https://www.terra.com.br/economia/subsidiaria-da-adyen-tem-aprovacao-do-bc-para-operar-como-instituicao-de-pagamento,99894c4c9eb81205eecebb52bca66b836bcfis38.html — (secondary) BACEN authorisation 2023-11-23, three modalities
- **[ADY-18]** https://docs.adyen.com/platforms/verification-requirements/ (via r.jina.ai) — legal-entity table incl. Brazil row; requirement factors
- **[ADY-20]** Search results docs.adyen.com (payouts/sweeps) — (search snippet only) push sweeps, sweep options, available-balance check, balance platform per region
- **[ADY-21]** https://docs.adyen.com/marketplaces/settle-funds (via r.jina.ai) — sales-day settlement model, settlement delay, reversed settlement 30 days
- **[ADY-22]** https://docs.adyen.com/marketplaces/split-transactions/split-refunds (via r.jina.ai) — refund split mirroring rule
- **[ADY-23]** https://docs.adyen.com/development-resources/webhooks/ — webhooks overview, event families, sub-pages
- **[ADY-24]** https://docs.adyen.com/online-payments/capture/ — capture modes, endpoint, multiple partial captures
- **[ADY-26]** https://github.com/Adyen/adyen-go-api-library — official Go library v21.2.2, APIs covered
- **[ADY-27]** https://docs.adyen.com/development-resources/live-endpoints/ — test/live URLs, prefix, timeouts
- **[ADY-28]** https://docs.adyen.com/development-resources/testing/ — test Customer Area credentials, webhook testing
- **[ADY-29]** https://docs.adyen.com/development-resources/api-idempotency/ — idempotency-key header, 7–14 days
- **[ADY-30]** https://docs.adyen.com/development-resources/libraries/ (via r.jina.ai) — library language tabs incl. Go

