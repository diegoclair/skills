# Aliases — Natural Language → Knowledge Base Category

Aliases are **natural language patterns** (keywords the team uses day-to-day) mapped to the **corresponding category** in the knowledge base. Use them as a routing shortcut before going to a CQL search.

> **Important:** this file lists **generic keyword patterns** (e.g. "advogado" → Advisors > Jurídico). The **specific mappings** (names of people, named companies, specific vendors) live in Confluence and must be obtained via bootstrap (see `bootstrap.md`) or CQL search.

> **How to use:** the user asks "onde registro um advogado?" → you look up "advogado" in this file → find "Advisors > Jurídico & Compliance" → fetch the current parentId via the Home (Page ID Index) or bootstrap. If the term isn't here, fall back to CQL.

---

## Keyword patterns → Category

| Keyword(s) | Target category |
|------------------|----------------------|
| advogado, jurídico, contrato, compliance societário, equity, exit | Advisors > Jurídico & Compliance |
| contador, fiscal, tributário | Advisors > Jurídico & Compliance (if external person) **OR** Operações > Contabilidade & Jurídico (if contracted service) |
| advisor, consultor, mentor, conselheiro | Advisors & Consultores (choose dept) |
| growth advisor, captação, performance mkt, performance, CRM, scale-up, aquisição de cliente | Advisors > Growth & Captação |
| e-commerce advisor, marketplace advisor | Advisors > Growth & Captação |
| varejista, grande varejista, target B2B, cliente B2B, integração B2B, e-commerce alvo, marketplace alvo | Parceiros > Grandes Varejistas |
| fornecedor tech, provider, API externa, integração técnica | Parceiros > Fornecedores Tech |
| KYC provider, validação de identidade, onboarding KYC (provedor) | Parceiros > Fornecedores Tech (Autenticação + KYC) |
| gateway, gateway de pagamento, adquirência, processador | Parceiros > Fornecedores Tech (Pagamentos) |
| orquestração de pagamento, vault, tokenização | Parceiros > Fornecedores Tech (Vault + Pagamentos) |
| device fingerprint, device intelligence, antifraude provider | Parceiros > Fornecedores Tech |
| face match, face matching, liveness, selfie biométrica | Parceiros > Fornecedores Tech (FaceMatch) |
| investidor, fundo, VC, venture capital, angel, captação de investimento, cheque | Parceiros > Investidores |
| aceleradora, incubadora, venture builder, programa de aceleração, early-stage program | Aceleração |
| concorrente, competidor, competitor, rival, competição, player similar, wallet concorrente | Research > Competidores |
| pesquisa com usuário, user research, form, entrevista, teste de usabilidade, UX research | Research > User Research |
| pesquisa consumidor, pesquisa lojista, survey | Research > User Research |
| fraude, fraudes, antifraude, chargeback, cartão clonado, golpe online | Research > Temas & Dados |
| carrinho abandonado, abandono de checkout | Research > Temas & Dados |
| open finance, open banking | Research > Temas & Dados |
| biometria, biométrico, reconhecimento facial | Research > Temas & Dados |
| segurança digital, cyber, cybersecurity | Research > Temas & Dados |
| payment gateway (como tema de estudo, não como fornecedor) | Research > Temas & Dados |
| ferramenta, SaaS, assinatura, software contratado, ferramenta interna, custo de software | Operações > Ferramentas & SaaS |
| escritório, coworking, sala física, endereço fiscal | Operações > Escritório & Coworking |
| contabilidade contratada, escritório contábil, jurídico contratado | Operações > Contabilidade & Jurídico |
| time, estrutura, org chart, funcionários, sócios | Operações > Time & Estrutura |
| AWS, infraestrutura, cloud, servidor, infra | Operações > Infraestrutura |
| custo, despesa operacional, opex, custo mensal | Operações > Custos Operacionais |
| KYC interno, onboarding de usuário Lybel (processo operacional) | Operações > KYC & Onboarding |
| feature, spec, funcionalidade, produto novo, roadmap | Produto |
| cashback, cash-back | Produto > Loops de crescimento |
| voucher, cupom Lybel | Produto > Loops de crescimento |
| member get member, MGM, indicação de usuário | Produto > Loops de crescimento |
| merchant get member, MGMerchant | Produto > Loops de crescimento |
| compra online, checkout online | Produto > Fluxos de compra |
| compra presencial, compra física, loja física | Produto > Fluxos de compra |
| compra recorrente, assinatura, Netflix-like | Produto > Fluxos de compra |
| shopping, shopping parceiros, vitrine | Produto > Fluxos de compra |
| cartão, cadastro de cartão, cadastramento automático | Produto > Cartão & Pagamento |
| compartilhamento de cartão | Produto > Cartão & Pagamento |
| banking, conta Lybel, serviços financeiros Lybel | Produto > Serviços financeiros |
| persona, JTBD, jobs to be done, mapa de empatia, proposta de valor | Produto (auxiliares) |
| princípios, valores, manifesto | Estratégia > Princípios |
| business model, BMC, business model canvas, modelo de negócio | Estratégia > BMC |
| growth strategy, estratégia de crescimento | Estratégia > Growth Strategy |
| financial, financeiro estratégico, projeção, valuation | Estratégia > Financial |
| fase 1, aquisição de clientes, GTM fase 1 | Estratégia > Lybel Fase 1 |
| pitch, pitch deck, apresentação | Estratégia > Pitch |
| sobre a Lybel, quem somos, overview, introdução, resumo oficial | Estratégia > Sobre a Lybel |

---

## Specific names (people, vendors, retailers, funds)

**Not in this file.** Mappings of specific names live in Confluence:
- Named people (specific advisors) → Home > Aliases section or Advisors table
- Specific vendors (KYC provider X, gateway Y) → Home > Aliases section or Fornecedores Tech
- Target retailers (names of marketplaces and e-commerces) → Análise de Parceiros
- Specific funds / accelerators → respective categories in Confluence

**Procedure when the user mentions a specific name:**

1. Run bootstrap (see `bootstrap.md`) — the Home has current aliases including proper names.
2. If the name doesn't appear on the Home, search with `mcp__atlassian__searchConfluenceUsingCql`:
   ```
   cql='space = "Lybel" AND (title ~ "NAME" OR text ~ "NAME") AND type = page'
   ```
3. If found, use the returned pageId.
4. If not found, **before creating a new page**, confirm with the user which category makes sense (applying the generic pattern from this table).

---

## Disambiguation tips

When the same term fits in two places, ask the user (or apply the rule):

- **"advogado"** → is it a person under evaluation/relationship? Advisors > Jurídico. Already a contracted firm in routine? Operações > Contabilidade & Jurídico.
- **"KYC"** → talking about an external provider? Fornecedores Tech. Talking about the internal user-onboarding process? Operações > KYC & Onboarding.
- **"contador"** → same rule as advogado.
- **"pagamento"** → external provider? Fornecedores Tech. Product feature (checkout)? Produto. Market-study theme? Research > Temas & Dados.
- **"aceleradora vs. investidor"** → structured acceleration program → Aceleração. Direct equity check → Parceiros > Investidores.
- **"varejista mencionado por nome"** → retailer we want to integrate (target B2B) → Parceiros > Grandes Varejistas. Not a competitor.

---

## When the term is NOT in the table

1. Apply the tie-breaking rules from `taxonomy.md`.
2. If still ambiguous, run `mcp__atlassian__searchConfluenceUsingCql` with `text ~ "term"` and let the result guide you.
3. If no result returns, ask the user which of the 6 categories makes sense — **do not invent a new category**.
4. After resolving, consider proposing an addition to the table above (continuous improvement of the skill).
