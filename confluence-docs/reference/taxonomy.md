# Lybel Knowledge Base Taxonomy

This document describes the **generic schema** of Lybel's knowledge base in Confluence. Use it as a structural reference before creating, moving or searching any page.

> **Important:** this is the generic schema (structure, rules, sub-types). The **current state** — specific names of people, companies, pages, statuses — always comes from the Home page in Confluence (pageId=164232). Always run bootstrap (see `bootstrap.md`) before acting, to get the current state. If this file diverges from the Home, **the Home wins**.

> **Golden rule:** before creating any new page, verify (1) whether it already exists (search), (2) which of the 6 categories below it fits in, (3) whether you need to create a new sub-grouping or reuse an existing one.

---

## The 6 categories

The Lybel knowledge base is organized into **6 top-level categories**. Every page must belong to one of them. Don't create a 7th category without prior discussion.

| # | Category | One-liner |
|---|-----------|----------|
| 1 | 💼 Estratégia & Visão | The "why" and "where to". |
| 2 | 📦 Produto | The "what". Features, specs, persona, roadmap. |
| 3 | 🤝 Parceiros & Relacionamentos | People and companies we do business with. |
| 4 | ⚙️ Operações | What makes Lybel run internally day-to-day. |
| 5 | 🔍 Research | External analysis — market, competitor, user, theme. |
| 6 | 🎓 Aceleração | Programs, funds and early-stage relationships. |

---

## 1. 💼 Estratégia & Visão

**Description:** Who we are, where we're going, how we monetize. Anything that changes per quarter+, not per week.

**What lives here (content types):**
- General vision / "About Lybel" (official company summary)
- Principles and values
- Business Model Canvas
- Growth strategy
- Financial planning / valuation / projections
- Phase plans (GTM, acquisition, expansion)
- Pitch / official decks

**Sub-structure:** flat. Pages are siblings under the category, with no required sub-levels. Brand, naming and design system documents may live in `/docs/brand/` in the Git repository (verify with the Home whether they have already been migrated).

---

## 2. 📦 Produto

**Description:** The "what". Features, specs, persona, roadmap. Always start with the problem it solves.

**Official sub-structure (feature categories):**
- **Fluxos de compra (core)** — user purchase journeys (online, in-store, recurring, shopping)
- **Cartão & Pagamento** — payment instrument management (registration, sharing)
- **Serviços financeiros** — banking, Lybel account, aggregated financial products
- **Loops de crescimento (viral / aquisição)** — acquisition/retention mechanics (cashback, voucher, MGM, MGMerchant)
- **Suporte** — problem resolution channels

**Persona/JTBD helpers (live in Produto, but are context):**
- Jobs To Be Done
- Value Proposition Canvas
- Persona and empathy map

**Rule for new feature:** always start with the problem (which pain we're solving, which journey we benefit). With the problem clear, purpose + technical decisions follow.

---

## 3. 🤝 Parceiros & Relacionamentos

**Description:** People and companies we do business with. **Four sub-types — don't mix.**

### 3.1 🏬 Grandes Varejistas (target B2B)
Brazilian target retailers we want to integrate. They are our B2B target customers.
- **What to document:** negotiation status, GMV, year-of-entry rationale (roadmap window), key contact, integration model
- **Common types:** B2B2C SaaS platforms, e-commerce & retail, marketplaces, niche players

### 3.2 🔌 Fornecedores Tech
API/infra providers Lybel consumes.
- **Common internal categories:** Pagamentos, Vault + Pagamentos, FaceMatch, Autenticação + KYC, Device Intelligence, Antifraude, Orquestração
- **What to document:** product, pricing, integration, status, alternatives evaluated

### 3.3 🧑‍⚖️ Advisors & Consultores
External individuals who help us (lawyers, consultants, accountants, advisors).
- **Organized by department.** Each department is a sub-page with a summary table and its advisors below.
- **Current and future departments:** consult the Home for the current state. Creation pattern: a new department is born only when there's a 1st person for it. Likely departments: Jurídico & Compliance, Growth & Captação, Financeiro, Tech, Produto & UX, Comercial & Parcerias B2B.

### 3.4 💰 Investidores
Funds, VCs and angels.
- **What to document:** fund, thesis, stage, average ticket, relevant portfolio, contact, conversation history

> **Current examples (specific names) are in Confluence in each section.** Always run bootstrap to know who is in each sub-type today.

---

## 4. ⚙️ Operações

**Description:** Everything that makes Lybel run internally day-to-day. If it changes monthly or is executed routinely, it's Operações.

**What lives here (content types):**
- Contracted Tools & SaaS
- Office & Coworking
- Contracted Accounting & Legal (routine)
- Team & Organizational structure
- Cloud infrastructure
- Operational costs (opex)
- KYC & Onboarding (internal Lybel user process)

**Sub-structure:** flat. Each operational domain is a sibling page.

> ⚠️ **Watch for overlap with Parceiros:** the *contracted accountant* goes in Operações > Contabilidade & Jurídico. The *lawyer under evaluation or in relationship* goes in Advisors > Jurídico & Compliance. Rule: if it's an **operational routine already contracted**, Operações; if it's an **external person in relationship**, Parceiros.

---

## 5. 🔍 Research

**Description:** External analysis — market, competitor, user, theme. Everything Lybel studies about the world outside it.

**Official sub-structure (3 sub-types):**

### 5.1 Competidores
- **What to document:** model, audience, strong point, weak point, where they beat us, comparative matrix

### 5.2 User Research
- **What to document:** objective, target audience, questions, method, results, insights
- **Common types:** problem research, usability test, interviews, surveys (consumer, retailer)

### 5.3 Temas & Dados
- **What to document:** market themes relevant to the business (fraud, chargeback, abandonment, open finance, biometrics, digital security, gateways as a theme, e-commerce antifraud)

---

## 6. 🎓 Aceleração

**Description:** Early-stage programs, funds and relationships. Accelerators, incubators, venture builders.

**What to document:** website, origin (how we met them), focus/thesis, status, next steps, materials.

**Status legend (mandatory on each sub-page):**
- 🟢 Em andamento — active relationship
- 🟡 Em contato — first approach made, awaiting reply
- 🔵 Pesquisada — evaluated but no contact yet
- ⚪ Sugestão — recommendation, not yet investigated
- 🔴 Descartada — doesn't make sense for Lybel today

> ⚠️ **Don't confuse with Parceiros > Investidores.** Aceleração = program/fund that accelerates an early-stage startup (structured program). Investidor = fund that writes a direct equity check. If it's a program, it goes in Aceleração. If it's a direct check, it goes in Parceiros > Investidores.

---

## Tie-breaking rules

When there's ambiguity, use:

1. **Competitor vs. Partner** — if we **compete** → Research > Competidores. If we want to **integrate/sell to them** → Parceiros.
2. **Operational vs. Strategic** — if it changes **monthly** → Operações. If it changes **per quarter+** → Estratégia.
3. **Product spec vs. Theme research** — if it's **what we're going to build** → Produto. If it's **what the market does** → Research.
4. **Contracted vs. under evaluation** — person/company **already contracted and operating** → Operações. Person/company **under evaluation or in relationship** → Parceiros.
5. **Accelerator vs. Investor** — program that accelerates (with curation, mentorship, structured program) → Aceleração. Fund that writes a direct check → Parceiros > Investidores.
6. **Person vs. Tech Company** — if it's an **individual** providing service/advice → Advisors. If it's a **company selling API/SaaS** → Fornecedores Tech.

---

## Structural IDs (root)

These IDs are structural (non-data) and may be fixed here. For any other pageId (sub-pages, people, companies), **read from the Home** or run a CQL search — don't trust hardcoded IDs outside this table.

| Resource | Value |
|---------|-------|
| Cloud ID | ab1dada3-b25e-40ad-9dbc-682caeea8d00 |
| Space | Lybel |
| Home (KB root) | pageId=164232 |

> **Always prefer reading fresh IDs from the Home.** The Home keeps the "Page ID Index" up to date — it is the canonical source of IDs for categorical parents and sub-categories.

---

## What NOT to do

- ❌ Create a loose page at the space root with no defined parent
- ❌ Duplicate an Advisor in Operações (advisor goes in Advisors; contracted service goes in Operações)
- ❌ Place a competitor in Parceiros — competitor is Research
- ❌ Create a 7th category without discussion
- ❌ Use the old name "SmartBuy" or "Qompra" — the current brand is **Lybel**
- ❌ Trust a specific pageId without confirming on the Home (leaf-page IDs may change/disappear)
