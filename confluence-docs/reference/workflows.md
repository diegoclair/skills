# Workflows — Step-by-Step to Operate the Lybel KB

This file defines deterministic flows. When the user asks for one of the actions below, execute the steps **in order, without skipping**. Ask the user **only** when the workflow indicates so explicitly.

**Conventions:**
- `cloudId` = `ab1dada3-b25e-40ad-9dbc-682caeea8d00` (always the same).
- Space = `Lybel` (key `lybel`, ID `131352`).
- **Tool priority:** prefer the Go CLI (`confluence-docs ...`) over the Atlassian MCP. The CLI returns sub-KB summaries; MCP returns full ADF bodies (10–40 KB). See SKILL.md §"Tool preferences" for the canonical table.
- **Read a page (3-step cascade):** `page digest` (outline + Status field) → `page get --section "Y" --format text` (one section as plain text) → `page get --format text` (whole page) → `page get --format export_view` / MCP `getConfluencePage(markdown)` only as fallback. Never load the whole ADF when you just need one section.
- **"Qual o status de X?"** is usually answered by the `Status:` line in `page digest` output alone (Lybel pages prefix titles with status emojis 🟢🟡🟠🔴🔵⚪✅ which the digest parses).
- **Create a page:** `confluence-docs page create --space-id 131352 --parent-id PID --title T --markdown FILE`. Fallback: MCP `createConfluencePage`.
- **Update a section:** `confluence-docs page apply --page-id ID --replace-section "Heading" --fragment FILE` (atomic GET→edit→PUT with 409 retry). Fallback: `page get` + `edit` + `page upload`. Last resort: MCP `getConfluencePage(adf)` + `updateConfluencePage(adf)`.
- **Rename / move a page:** `confluence-docs page move --page-id ID [--parent-id NEW_PARENT] [--title NEW_TITLE]` — body preserved, single PUT. At least one of `--parent-id` / `--title` is required.
- **Delete a page:** `confluence-docs page delete --page-id ID --yes` — soft delete (Confluence trash, restorable). Always confirm with the user before issuing.
- **Search:** `confluence-docs search "term" --limit 5`. Fallback: MCP `searchConfluenceUsingCql`.
- **PageIds of parents (categories, sub-categories, departments)** come from the Home (Page ID Index) — always run Workflow 0 before other workflows in the session.

---

## Workflow 0 — Bootstrap (always run at session start)

**Trigger:** first interaction of the session involving the Lybel KB.

**Why it exists:** this skill is timeless — it knows generic structure/rules, but **not the current state** (who the advisors are today, which accelerators are in progress, which investor is in conversation, current pageIds of each parent). That state lives on the Confluence Home and must be read fresh in each session.

**Steps:**

1. **Just query the cache directly — it auto-refreshes when stale:**
   ```
   confluence-docs home --query "<term>"   # alias / decision-map lookup
   confluence-docs home --digest           # outline + heading list
   confluence-docs home --show             # full text rendering
   ```
   These commands handle freshness internally: if the cache file is missing or older than 1h, they auto-fetch from Confluence and serve. You don't have to track when to refresh.

2. **Extract from the Home:**
   - **"Onde coloco X?" table** — current decision map for routing
   - **"Aliases" section** — keywords → specific pages (including current proper names)
   - **"Page ID Index" section** (if present) — IDs of categorical parents / departments / sub-categories
   - **Categories with current links** — current state of the 6 categories

3. **Use this data as source of truth** for the rest of the conversation. If it conflicts with `taxonomy.md` / `aliases.md` / `templates.md`, **the Home wins**.

4. **Writes auto-refresh the cache.** When you call `page apply` / `index add` / `index remove` / `index sync` and the target is the Home page (pageId 164232), the cache is automatically re-fetched after the PUT succeeds. You don't need to refresh manually after writes.

5. **The cache is shared across all Claude sessions on the same machine.** If one session refreshes (manually or via a write), every other session reading next sees the new state.

6. **`home --refresh`** exists only for the rare "another machine just updated; I don't want to wait for the 1h TTL" case. It always fetches.

7. **Fallback:** if the CLI is unavailable, use MCP `getConfluencePage(pageId="164232", contentFormat="markdown")` and operate from in-memory data. If both are inaccessible, use the static files:
   - `taxonomy.md` (generic structure)
   - `aliases.md` (generic keyword patterns)
   - `templates.md` (formats)
   - `bootstrap.md` (this principle)

   Warn the user: "Home indisponível, operando em modo degradado com schema genérico — pageIds de parents podem estar desatualizados."

**Cache safety invariant:** the cache is **read-only for navigation**. Writes to the Home (or any page) ALWAYS go through `page apply`, which GETs fresh ADF from Confluence before any PUT. The cache is never used as the source for an update — this is what protects against overwriting work someone did on a different machine.

---

## Workflow 1 — Add new lawyer / consultant / advisor

**Trigger:** user says "adicionar advogado", "cadastrar consultor", "novo advisor", "registrar [nome] como advisor", etc.

**Steps:**

1. **Ensure bootstrap.** If you haven't done it in this session, run Workflow 0.

2. **Identify the area.** Ask the user:
   > "Qual a área de atuação dessa pessoa? Jurídico & Compliance, Growth & Captação, ou outra área?"
   - If existing area → use the department's pageId (obtained via Home Page ID Index).
   - If **another area** → ask:
     > "Não temos esse departamento ainda. Quer que eu crie um novo departamento (ex: 💰 Financeiro, 🔧 Tech, 📦 Produto & UX, 🤝 Comercial)? Se sim, qual nome e emoji?"
     - If yes → create department sub-page under the parent Advisors & Consultores (pageId via Home) with title format `(Emoji) (Nome do Depto)`, then use this parentId in step 4.
     - If no → don't create. End and suggest the user choose one of the existing depts.

3. **Collect minimum data.** Ask (a single round):
   - Full name
   - Specialty / function
   - LinkedIn (if available)
   - Origin of the contact (how we met)
   - Reason why it interests Lybel

4. **Create the page.** Call `mcp__atlassian__createConfluencePage`:
   - `title`: `(Função) - (Nome)`
   - `parentId`: the dept identified in step 2
   - `body`: Advisor template (see `templates.md` §1) filled with the collected data
   - Initial status: 🟡 Em avaliação (unless the user specifies otherwise)

5. **Update the summary table on the parent Advisors & Consultores.**
   - Fetch the page with `mcp__atlassian__getConfluencePage` in `markdown` format.
   - In the "📋 Visão geral" table, add a row: `| [Nome](url) | (emoji+área) | (status emoji+texto) | (especialidade curta) |`.
   - Update with `mcp__atlassian__updateConfluencePage`.

6. **Return to the user:**
   - URL of the created page
   - Confirmation "Tabela de Advisors atualizada"
   - Suggested next action (e.g. "quer adicionar e-mail/telefone?")

---

## Workflow 2 — Add partner (Grande Varejista)

**Trigger:** "adicionar varejista", "novo parceiro B2B", "cadastrar [nome de varejista]".

**Steps:**

1. **Ensure bootstrap.**

2. **Validate it's actually a retailer.** If the name suggests ambiguity (could it be a competitor?), apply the tie-breaking rule from `taxonomy.md` §Rules: if we want to integrate/sell → Parceiros.

3. **Parent:** Análise de Parceiros (pageId via Home Page ID Index).

4. **Collect minimum data** (a single round):
   - Official name
   - Category (Plataforma SaaS B2B2C | E-commerce & Varejo | Marketplace | Nicho)
   - Estimated annual GMV (if known)
   - Current negotiation status
   - Entry rationale (why we want to integrate)

5. **Create page** with `mcp__atlassian__createConfluencePage`:
   - `title`: `(Nome) - Financial Analysis` (current parent's standard) OR `(Nome) - Análise de Parceria`.
   - `parentId`: pageId of Análise de Parceiros
   - `body`: Grande Varejista template (see `templates.md` §3).

6. **Update strategic table** on the Análise de Parceiros page if the retailer enters the current roadmap window.

7. **Return** URL and recap.

---

## Workflow 3 — Add new accelerator

**Trigger:** "adicionar aceleradora", "nova aceleração", "registrar [nome de programa de aceleração]".

**Steps:**

1. **Ensure bootstrap.**

2. **Parent:** Aceleração (pageId via Home Page ID Index).

3. **Collect minimum data** (a single round):
   - Official name
   - Website (if known)
   - Origin (referral from whom, event, cold)
   - Initial status (🟢/🟡/🔵/⚪/🔴 — if unknown, default 🔵 Pesquisada)
   - Focus/thesis (optional)

4. **Create page** with `mcp__atlassian__createConfluencePage`:
   - `title`: official accelerator name
   - `parentId`: pageId of Aceleração
   - `body`: Accelerator template (see `templates.md` §2).

5. **Update "📊 Status atual" table** on Aceleração:
   - Fetch in markdown → add row `| [Nome](url) | (status emoji + texto) | (origem) | (observação) |` → update.

6. **Return** URL + confirmation that the table was updated.

---

## Workflow 4 — Search page by theme

**Trigger:** "onde está X?", "tem página sobre Y?", "me mostra o que tem sobre Z".

**Steps:**

1. **Ensure bootstrap.** The Home brings current aliases with proper names — usually resolves directly.

2. **Check aliases (Home + `aliases.md`).** If the term (or close synonym) is mapped, return the indicated page/parent directly.

3. **If not in aliases**, run the CLI search (preferred):
   ```
   confluence-docs search "TERM" --limit 10
   ```
   This expands to `space = "lybel" AND type = "page" AND (title ~ "TERM" OR text ~ "TERM")`. Output is TSV (`pageId<TAB>title<TAB>url<TAB>excerpt`).
   - For raw CQL control: `confluence-docs search --cql 'space=lybel AND label="adr"'`.
   - Try pt-BR variants (with/without accents): "fraude" and "fraudes", "investidor" and "investidora".

   **Fallback** (CLI unavailable):
   ```
   mcp__atlassian__searchConfluenceUsingCql(
     cloudId="ab1dada3-b25e-40ad-9dbc-682caeea8d00",
     cql='space = "lybel" AND (title ~ "TERM" OR text ~ "TERM") AND type = page',
     limit=10
   )
   ```

4. **Filter results** to the expected category, if the user gave a hint (e.g. "advogado do time" → prioritize pages under Advisors & Consultores).

5. **Return to the user**, for each hit:
   - Title
   - Full URL
   - Short excerpt (response summary) — 1-2 lines
   - pageId (useful for subsequent actions)

6. **If zero results**, answer honestly:
   > "Não encontrei página sobre '(termo)'. Pelas aliases, esse tema caberia em (categoria sugerida). Quer que eu crie uma página nova lá?"

---

## Workflow 5 — List things by status

**Trigger:** "quais aceleradoras em andamento?", "advisors ativos", "varejistas em negociação", "fornecedores contratados".

**Steps:**

1. **Ensure bootstrap.**

2. **Identify the category and the summary table.** Each category has its status table on the parent:
   - Aceleradoras → Aceleração page, "📊 Status atual" table
   - Advisors → Advisors & Consultores page, "📋 Visão geral" table
   - Varejistas → Análise de Parceiros page, strategic roadmap table
   - Investidores → navigate sub-pages under Parceiros > Investidores (there may not be a consolidated summary table — confirm via Home)

3. **Fetch the parent** in `markdown` format:
   ```
   mcp__atlassian__getConfluencePage(cloudId=..., pageId=PARENT_ID, contentFormat="markdown")
   ```

4. **Parse the table.** Locate the heading (`## 📊 Status atual` or similar) and extract the rows that follow. Each row = entity + status.

5. **Filter by the requested status.** E.g. user asked "em andamento" → keep only rows with `🟢` or text "Em andamento".

6. **Return formatted answer:**
   ```
   Aceleradoras em andamento (N):
   - [Nome] — (observação curta da tabela)
     https://lybel.atlassian.net/wiki/spaces/lybel/pages/PAGE_ID
   ```

7. **If the category has no summary table**, use CQL fallback:
   ```
   cql='parent = PARENT_ID AND text ~ "STATUS_EMOJI OR STATUS_TEXTO"'
   ```

---

## Workflow 6 — Add tech vendor

**Trigger:** "adicionar fornecedor", "cadastrar [nome de fornecedor]", "novo KYC provider", "gateway novo".

**Steps:**

1. **Ensure bootstrap.**

2. **Parent:** Fornecedores Tech (pageId via Home Page ID Index).

3. **Confirm internal category** (if not obvious): Pagamentos / Vault+Pagamentos / FaceMatch / Autenticação + KYC / other. If other, suggest creating a new section on the parent.

4. **Collect:** website, category, product/service (1-3 bullets), estimated pricing, integration type, status.

5. **Create page** with the Tech Vendor template (see `templates.md` §4), `parentId` = Fornecedores Tech.

6. **Update the parent** by adding a link in the correct categorical section.

7. **Return** URL + suggested next steps (e.g. "quer agendar call de sandbox?").

---

## Workflow 7 — Add investor

**Trigger:** "adicionar investidor", "novo fundo", "cadastrar [nome de fundo / VC / angel]".

**Steps:**

1. **Ensure bootstrap.**

2. **Parent:** under Parceiros > Investidores. If there's no dedicated hub page, investors may be direct children of the Parceiros area — confirm via Home. When in doubt, ask the user where to anchor.

3. **Title:** `Investor - (Nome do Fundo)`.

4. **Collect:** official name, website, thesis, stage, average ticket, relevant portfolio, contact, how we got there.

5. **Create page** with the Investor template (see `templates.md` §6).

6. **Return** URL.

---

## Workflow 8 — Create product feature

**Trigger:** "nova feature", "adicionar feature (X)", "documentar feature", "spec de (funcionalidade)".

**Steps:**

1. **Ensure bootstrap.**

2. **Identify sub-category in Produto** (ask if not obvious):
   - Fluxos de compra (core)
   - Cartão & Pagamento
   - Serviços financeiros
   - Loops de crescimento (viral / aquisição)
   - Suporte

3. **Reinforce the central rule:** ask the user "**qual o problema que essa feature resolve?**" before anything else. If the user can't answer, stop and align — Lybel Produto always starts with the problem.

4. **Collect:** feature name, problem, value proposition (consumer/retailer/Lybel), expected flow, key decisions, out of scope.

5. **Create page** with the Feature template (see `templates.md` §7). `parentId` = Produto or the appropriate sub-category (pageId via Home).

6. **Return** URL + suggest adding the complementary technical page (when applicable).

---

## Workflow 9 — User Research

**Trigger:** "nova pesquisa", "documentar entrevista", "adicionar form de usuário".

**Steps:**

1. **Ensure bootstrap.**

2. **Parent:** User Research (pageId via Home Page ID Index).

3. **Collect:** objective, target profile, method, sample size, fieldwork date.

4. **Create page** with the User Research template (see `templates.md` §5). Title format `(Tipo) - (Público) - YYYY-MM`.

5. **If results already exist**, fill the "Resultados" and "Insights" sections. If not, leave them empty and status in history as "pesquisa lançada".

6. **Return** URL.

---

## Workflow 10 — Determine category when ambiguous

**Trigger:** user asks to create something without knowing where. "Onde coloco essa coisa X?"

**Steps:**

1. **Ensure bootstrap.** The Home has the "Onde coloco X?" table — current decision map.

2. **Check aliases (Home + `aliases.md`)** for the term. If found, answer directly.

3. **Apply tie-breaking rules** (from `taxonomy.md` §Rules):
   - Competitor vs. Partner
   - Operational vs. Strategic
   - Spec vs. Research
   - Contracted vs. under evaluation
   - Accelerator vs. Investor
   - Person vs. Company

4. **If still ambiguous**, present to the user the 1-2 most likely options with short rationale and ask them to pick:
   > "Posso colocar em (A) porque (motivo) ou (B) porque (motivo). Qual faz mais sentido?"

5. **Never invent a 7th category.** The 6 are immutable without explicit discussion with the user.

---

## Cross-cutting execution rules

- **No mandatory bootstrap step** in a session. Just use `home --query/--show/--digest` directly — they auto-refresh when stale. Per-session bookkeeping is gone.
- **Use the cache for navigation, not API calls.** `home --query "X"` is local and free (auto-refreshes when needed); reach for `page digest` only when the term isn't in the Home and you need to inspect a specific page.
- **Prefer `digest` to full reads.** Most "qual o status / o que tem em X" questions are answered by the headings outline alone.
- **For updates, prefer `page apply`** — atomic GET → edit → PUT, with macro preservation and a 409-conflict retry. Supports section ops (`--replace-section`, `--append`, `--insert-after/before`, `--delete-section`) and table ops (`--table-add-row`, `--table-remove-row`). Falls back to `page get` + `edit` + `page upload` only if you need to compose multiple ops in one PUT.
- **After mutating the Home, refresh the cache** (`home --refresh`) so subsequent local queries see the new state.
- **Never use `contentFormat="markdown"` on a page with macros** (TOC, expand, panel, status). It silently flattens the structure on update. The CLI uses ADF round-trips internally and never has this risk.
- **Always update the parent's summary table** when creating a sub-page in a category that has a table (Advisors, Aceleração, Varejistas). Use `page apply --table-add-row "Heading" --row "col1|col2|..." --if-missing` for a single-shot atomic update. (The Home's Page ID Index has a dedicated wrapper: `index add`.)
- **Always include `## Histórico`** with the creation date on new pages.
- **Never reference "SmartBuy" or "Qompra"** in new pages — the brand is **Lybel**. If you find old pages with these names, flag to the user but **do not edit without authorization**.
- **When in doubt, ask once** and proceed. Don't stack questions — the Lybel team prefers action with explicit assumptions over a long interrogation.
- **Use placeholders** (e.g. `[Nome do Advisor]`) when the user doesn't know a field — don't invent data.
- **Respect the status emojis** of each category — they're part of the visual convention:
  - Aceleração: 🟢🟡🔵⚪🔴
  - Advisors: 🟢🟡🔴
  - Varejistas: emoji-free with text (MVP/Expansão/Escala) or 🔵🟡🟢✅🔴
  - Fornecedores: 🔵🟡🟢🔴
