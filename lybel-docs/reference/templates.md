# Lybel Confluence Page Templates

When creating a new page under a category, use the corresponding template below. **Copy the markdown as-is** and fill in the fields between parentheses or brackets. Keep emojis and structure — the Lybel team recognizes the pattern visually.

> **Templates are structural.** Fill them with current data of the moment. Use placeholders (e.g. `[Nome do Advisor]`, `[Nome da Banca]`, `[Nome do Fundo]`) for unknown fields — the partner/Diego completes them later. **Don't invent data** to fill a placeholder.

> **ParentId:** every template mentions the parent type (e.g. "Advisors department"). The **concrete pageId** comes from the Home (Page ID Index) or bootstrap — it isn't hardcoded here.

> **General rule:** every new page ends with a `## Histórico` section containing at least the creation date. This is sacred — it makes auditing and temporal context easier.

---

## Universal page header (apply to every new page)

Every page starts with a small header block (quote/callout) BEFORE any content heading:

```markdown
> **Contexto:** (one-line summary of what the page is about)
> **Criado em:** YYYY-MM-DD | **Atualizado em:** YYYY-MM-DD | **Criado por:** [nome]
```

Example:

```markdown
> **Contexto:** Proposta de MVP da Fase 1 da Lybel.
> **Criado em:** 2026-04-08 | **Atualizado em:** 2026-04-24 | **Criado por:** Diego C. Rodrigues
```

**Rules**:
- Apply to all pages EXCEPT index/reference/glossary pages
- When updating: bump "Atualizado em" date
- When creating: confirm author if unknown (don't invent)

---

## 1. Template — Advisor / Consultor (individual)

**Use when:** adding a lawyer, accountant, consultant, advisor or any external person who helps us.
**Parent:** department sub-page under Advisors & Consultores (e.g. Jurídico & Compliance, Growth & Captação, or a new dept).

**Page title:** `(Função) - (Nome)` — e.g. "Advogado societário - [Nome do Advisor]", "Growth Advisor - [Nome do Advisor]".

```markdown
## Contato

* **Nome**: [Nome completo]
* **LinkedIn**: [handle](url)
* **Email / telefone**: [adicionar]

## Especialidade

* (bullet principal)
* (bullets complementares)
* (experiência relevante)

## Status

🟡 Em avaliação

*(usar: 🟢 Ativo | 🟡 Em avaliação | 🔴 Arquivado)*

## Por que nos interessa

(1-3 parágrafos explicando por que essa pessoa é relevante pra Lybel — que fase/problema ela resolve)

## Histórico

* YYYY-MM-DD: (evento — ex: identificado via podcast, primeira reunião, indicação de X)
```

**After creating:** update the "Visão geral" table on the parent Advisors & Consultores with the new row.

---

## 2. Template — Accelerator / Early-Stage Fund

**Use when:** adding an accelerator, incubator, venture builder or early-stage program.
**Parent:** Aceleração index page.

**Title:** official accelerator name — e.g. "[Nome do Programa] ([Sigla])".

```markdown
## Sobre

* **Site**: (url)
* **Origem**: (como conhecemos — indicação de quem, evento, etc.)
* **Status**: (um dos da legenda abaixo)
* **Foco/tese**: (que tipo de startup eles aceleram)

## Por que faz sentido pra Lybel

(parágrafo curto — em que essa aceleradora pode acelerar a Lybel especificamente)

## Próximos passos

- [ ] Ação 1
- [ ] Ação 2
- [ ] Data da próxima reunião / marco

## Histórico

* YYYY-MM-DD: evento relevante

## Materiais

* Video pitch: (link Drive)
* Slides: (link quando disponível)

## Contatos

(nomes, emails, telefones conforme o relacionamento evolui)
```

**Status legend (paste into Status):**
🟢 Em andamento | 🟡 Em contato | 🔵 Pesquisada | ⚪ Sugestão | 🔴 Descartada

**After creating:** update the "📊 Status atual" table on the Aceleração index page with the new row.

---

## 3. Template — Grande Varejista (target B2B)

**Use when:** adding a B2B target retailer (marketplace, SaaS platform, mid/high-ticket e-commerce).
**Parent:** Análise de Parceiros index page (under Parceiros > Grandes Varejistas).

**Title:** `(Nome) - Financial Analysis` OR `(Nome) - Análise de Parceria`.

```markdown
## Visão Geral

* **Site**: (url)
* **Categoria**: (Plataforma SaaS B2B2C | E-commerce & Varejo | Marketplace | Nicho)
* **GMV anual**: (R$ X Bi — fonte)
* **Transações/ano**: (N milhões)
* **Lojistas / base ativa**: (N)
* **Status da negociação**: 🔵 Pesquisado / 🟡 Em contato / 🟢 Em andamento / ✅ Fechado / 🔴 Descartado

## Por que é alvo estratégico

(1-3 bullets — rationale de por que queremos integrar)

## Proposta de integração

* **Modelo**: (app, plugin, API direta, SDK)
* **Proposta de valor pro varejista**: (conversão+, fraude-, UX+)
* **Take rate proposto**: (% ou faixa)
* **Janela de entrada**: (Ano 1 / 2 / 3... conforme plano do roadmap vigente)

## Contato-chave

* **Nome**: [Nome do Contato]
* **Cargo**: (posição)
* **Como chegamos**: (indicação, LinkedIn, evento)
* **Email**: (se disponível)

## Próximos passos

- [ ] Ação
- [ ] Ação

## Histórico

* YYYY-MM-DD: (pitch enviado, reunião, etc.)

## Materiais

* (deck da abordagem, estudo financeiro, etc.)
```

---

## 4. Template — Tech Vendor

**Use when:** adding an API/infra provider (KYC, payment, vault, antifraud, device intelligence).
**Parent:** Fornecedores Tech index page (under Parceiros).

**Title:** vendor name — e.g. "[Nome do Fornecedor]".

```markdown
## Quem é

* **Site**: (url)
* **Categoria**: (Pagamentos | Vault | KYC | FaceMatch | Antifraude | Device Intelligence | Orquestração)
* **HQ / país**: (onde operam)
* **Anos de mercado**: (N)

## Produto / Serviço

(O que eles fazem em 1-3 bullets técnicos)

* Feature chave 1
* Feature chave 2
* Feature chave 3

## Pricing

* **Modelo**: (por transação / mensalidade / setup+MRR / % do GMV)
* **Valor estimado**: (R$ ou US$)
* **Volume mínimo**: (N transações / mês)
* **Setup fee**: (sim/não, quanto)

## Integração

* **Tipo**: (REST API / SDK mobile / webhook / SaaS console)
* **Complexidade estimada**: (baixa / média / alta)
* **Docs**: (link)
* **Sandbox**: (sim/não, link)

## Decisão

* **Status**: (🔵 Avaliando | 🟡 POC | 🟢 Contratado | 🔴 Descartado)
* **Rationale**: (por que sim ou por que não)
* **Alternativas consideradas**: (lista de concorrentes diretos avaliados)

## Contato comercial

* **Nome**: [SDR/AE]
* **Email**: [adicionar]

## Histórico

* YYYY-MM-DD: (primeira call, demo, proposta)
```

---

## 5. Template — User Research (interview / form)

**Use when:** documenting research with a consumer or retailer.
**Parent:** User Research index page (under Research).

**Title:** `(Tipo) - (Público) - YYYY-MM` — e.g. "Pesquisa do problema - Consumidor - YYYY-MM".

```markdown
## Objetivo

(1 parágrafo: o que queremos aprender e por quê)

## Perfis-alvo

* (quem respondeu / quem vamos entrevistar)
* Critérios de inclusão: (idade, renda, comportamento de compra)
* Critérios de exclusão: (se aplicável)

## Método

* **Tipo**: (form online / entrevista 1:1 / teste de usabilidade moderado / diary study)
* **Canal**: (Google Forms, Typeform, Zoom, presencial)
* **UTM_Source**: (específico por pesquisa)
* **N amostral**: (N alvo / N obtido)
* **Data de campo**: (YYYY-MM-DD a YYYY-MM-DD)

## Perguntas

1. Pergunta 1
   1. Alternativa
   2. Alternativa
2. Pergunta 2
   1. Alternativa
...

## Resultados

(dados crus, tabelas, gráficos ou link pro dashboard)

## Insights

* **Insight 1** — (implicação pro produto)
* **Insight 2** — (implicação pra estratégia)
* **Insight 3** — (dor validada / invalidada)

## Próximos passos

- [ ] Ação derivada do insight
- [ ] Nova hipótese a validar

## Histórico

* YYYY-MM-DD: pesquisa lançada
* YYYY-MM-DD: consolidação de resultados
```

---

## 6. Template — Investor

**Use when:** adding a fund, VC or angel.
**Parent:** Parceiros > Investidores (under the Parceiros category).

**Title:** `Investor - [Nome do Fundo]`.

```markdown
## Sobre

* **Nome do fundo**: [Nome oficial]
* **Site**: (url)
* **Crunchbase / Tracxn**: (link)
* **HQ**: (país/cidade)

## Tese

* **Estágio**: (pre-seed | seed | Series A | growth)
* **Ticket médio**: (US$ X — Y)
* **Setores**: (fintech, retail tech, ...)
* **Geografia**: (Brasil, LatAm, global)

## Por que faz sentido pra Lybel

(parágrafo — thesis fit, portfolio de referência, value-add além do cheque)

## Portfolio relevante

* (empresa 1) — categoria
* (empresa 2) — categoria
* (mesmo setor ou complementar)

## Status

* **Status**: (⚪ Sugestão | 🔵 Pesquisado | 🟡 Em contato | 🟢 Em andamento | 🔴 Descartado)

## Contato

* **Nome**: [partner/analyst]
* **Email / LinkedIn**: (link)
* **Como chegamos**: (warm intro, cold, evento)

## Próximos passos

- [ ] Ação

## Histórico

* YYYY-MM-DD: primeiro contato
* YYYY-MM-DD: próximo marco
```

---

## 7. Template — Product Feature

**Use when:** creating a new page under Produto.
**Parent:** Produto index page → appropriate sub-category (Fluxos de compra, Cartão & Pagamento, Serviços financeiros, Loops de crescimento, Suporte).
**Central rule:** **always start from the problem.**

**Title:** short, specific feature name.

```markdown
## Problema que resolve

(1-3 parágrafos: qual dor, qual jornada, quem sofre hoje — consumidor? varejista? ambos?)

## Proposta de valor

* **Para o consumidor**: (1 frase curta)
* **Para o varejista**: (1 frase curta)
* **Para a Lybel** (modelo de receita ou loop): (1 frase curta)

## Fluxo esperado

(passo-a-passo da jornada ideal, do gatilho até o outcome)

1. Usuário faz X
2. Sistema Lybel faz Y
3. Varejista recebe Z
4. Outcome final

## Decisões-chave

* **Decisão 1**: (opção escolhida) — porque (rationale)
* **Decisão 2**: (opção escolhida) — porque (rationale)

## Fora do escopo

* (o que explicitamente NÃO vamos fazer nesta feature)

## Persona/JTBD envolvido

Referência: páginas de JTBD e Persona (links — buscar pageIds atuais via Home).

## Complementar técnico

(link para página técnica com specs, diagramas, API contracts — quando aplicável)

## Histórico

* YYYY-MM-DD: ideia registrada
* YYYY-MM-DD: spec consolidada
```

---

## 8. Template — Decision / Proposal / Strategy

**Use when:** documenting any decision, proposal, strategy, spec, risk analysis, or viability assessment. This is the **default template** for sub-pages of Estratégia & Visão, Produto specs, Fase/MVP pages, and any analytical/decision-making page.
**Parent:** whatever makes sense in the taxonomy (see Home).
**Central rule:** **Contexto → Problema → Solução**, always.

**Title:** short, specific — e.g. "MVP Lybel Pocket", "Risco: commoditização por protocolos agênticos", "Proposta de KYC ultra-leve".

```markdown
> **Contexto:** (one-line summary)
> **Criado em:** YYYY-MM-DD | **Atualizado em:** YYYY-MM-DD | **Criado por:** [nome]

## Contexto

(2-4 parágrafos — onde a página nasce, qual projeto/decisão/momento, qual o escopo maior em que ela vive. Se essa decisão está ligada a outra página, linka.)

## Problema

(O que está sendo resolvido, qual a dor, quais as restrições. Use bullets se ajudar a ler rápido.)

- Restrição 1
- Restrição 2
- Dor concreta

## Solução (possível)

(A proposta. Pode ser:
- Única e concreta — com escopo, stack, métricas
- Múltipla — comparativo de opções com tradeoffs

Se for única, detalha escopo/fluxo/decisões-chave.
Se for múltipla, tabela comparativa + recomendação.)

### Escopo (se solução única)
(...)

### Opções (se solução múltipla)

| Opção | Prós | Contras | Quando usar |
|---|---|---|---|
| A | | | |
| B | | | |

## Decisões abertas

- [ ] Pergunta 1 que precisa de resposta
- [ ] Pergunta 2 que precisa de resposta

## Referências

- [Página relacionada 1](url)
- [Página relacionada 2](url)

## Histórico

* YYYY-MM-DD: página criada
* YYYY-MM-DD: (próxima atualização significativa — descrever mudança)
```

**When NOT to use this template**: indices, glossaries, lists of contacts/partners/tools, pure reference pages. Those have their own templates above (Advisor, Investor, Tech Vendor, etc.) ou seguem formato de índice.

---

## Universal checklist before saving any page

- [ ] **Page header** com `Contexto`, `Criado em`, `Atualizado em`, `Criado por` (exceto índices/referências)
- [ ] **Contexto → Problema → Solução** se for página de decisão/proposta/estratégia
- [ ] **Jargão explicado ou linkado** (Merchant of Record, MCP, Variante A, v0.1, commission fee, postback — tudo que um leitor novo não entenderia deve ter uma linha ou um link)
- [ ] **Referências linkadas** — toda menção a outra decisão/análise/research deve linkar a página Confluence correspondente, não só citar
- [ ] Title follows the category convention (see each template above)
- [ ] Correct parent (check taxonomy.md / Home if in doubt)
- [ ] `## Histórico` section present with creation date
- [ ] Status uses standardized emojis (🟢🟡🔵⚪🔴 or 🟢🟡🔴 depending on category)
- [ ] No reference to old brands "SmartBuy" or "Qompra" — it's **Lybel**
- [ ] If you created a new person/company under a parent with a summary table, **you updated the parent's table**
- [ ] Placeholders (e.g. `[Nome do Advisor]`) are explicit where data is unknown — no invented data
