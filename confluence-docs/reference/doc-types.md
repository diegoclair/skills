---
type: reference
status: active
owner: "@confluence-docs"
tags: [editorial, doc-types, canonical-spec, skill-contract]
related: []
created: 2026-05-12
updated: 2026-05-12
---

# Doc Types: Canonical Spec for `confluence-docs`

## About this file

This is the canonical spec for any project using the `confluence-docs` skill. It defines the 5 document types, their required structure, frontmatter fields, naming conventions, cross-linking rules, and anti-patterns.

Projects MAY extend or override this with their own editorial guide (e.g., `docs/standards/EDITORIAL.md`), but the 5 types and frontmatter field names are the contract. The skill reads this file as its authoritative reference — `SKILL.md` and the `km` command derive their behavior from Sections 2–4 here.

---

## TL;DR

- Every page has mandatory frontmatter, a TL;DR if > 300 words, and at least 1 incoming link.
- There are 5 doc types: `reference`, `decision`, `explanation`, `how-to`, `capture`.
- Decisions with `status: accepted` are immutable — to change one, supersede it with a new doc.
- The skill reads this file as its canonical contract; no template logic is hardcoded in the skill binary.

---

## Section 1 — Principles: Why This Standard Exists

Knowledge bases grow along two axes simultaneously: human readers (founders, hires, collaborators) and AI agents (LLMs consuming the knowledge base as context). Writing "for humans first" produces prose that LLMs parse poorly — generic headers, implicit context, self-referential paragraphs. Writing "for AI first" produces docs nobody reads voluntarily. This standard resolves both.

The methodology draws from three references: the **Karpathy LLM Wiki Pattern** (Raw Sources / Wiki / Schema layers; plain markdown without a vector DB for small-to-medium wikis), **A-MEM / Zettelkasten for agents** (atomic notes, explicit links, rich metadata), and **Diátaxis adapted** (four documentation modes extended with a business-context layer). The result is a system where every page knows what it is, where it lives, what it relates to, and can be consumed self-sufficiently by any reader — human or machine.

**Principles:**

- **Index mandatory, orphan pages prohibited.** Every page must appear in the project's KNOWLEDGE_MAP and have at least 1 incoming link.
- **Frontmatter is a contract.** Without frontmatter, the page is not ready. It is not optional.
- **Descriptive headers, never generic.** `## Context: why Stripe was chosen over Braintree` — not `## Context`.
- **Accepted decisions are immutable.** Changing an accepted decision means creating a new decision doc that supersedes the previous one.
- **No process noise in the body.** Edit history belongs in the `updated` metadata field, not in body paragraphs like "rewritten after May review."

---

## Section 2 — The 5 Doc Types: Definitions, Structure, and Boundaries

| Type | What it is | Where it lives | Immutable? |
|---|---|---|---|
| `reference` | Stable facts about an external or internal entity | Confluence | No |
| `decision` | A recorded choice with context and consequences | Git `/docs/specs/` | **Yes** (once `accepted`) |
| `explanation` | Analysis, strategic context, research | Confluence | No |
| `how-to` | Step-by-step operational process | Confluence | No |
| `capture` | Raw idea or seed not yet shaped | Confluence (Ideas folder) | No |

### 2.1 `reference` — Stable Data About an Entity

**What it is:** Competitor profile, ICP, advisor, partner, investor; market data; adopted stack or tool.

**Required structure (in this order):**
1. Frontmatter (Page Properties macro on Confluence; YAML on Git)
2. TL;DR
3. `## Identity: <entity name and category>`
4. `## Attributes: <what defines this entity>`
5. `## Relevance: <why this entity matters to the project>`
6. `## Sources` (links, access dates)

**Generic examples:** ICP: Solo SaaS founder; Competitor: Stripe; Tool: PostgreSQL as primary datastore.

**When to use:** The entity has its own identity, is referenced by other docs, and needs a single source of truth.

**When not to use:** If the content is a comparative analysis across multiple entities, use `explanation`. If it records a decision to adopt the entity, use `decision`.

---

### 2.2 `decision` — A Recorded, Immutable Choice

**What it is:** ADR (Architecture Decision Record), pivot record, payment provider selection, business model definition.

**Required structure (in this order):**
1. Frontmatter
2. TL;DR
3. `## Problem: <what needed to be decided>`
4. `## Context: <pressures, constraints, and information available at decision time>`
5. `## Decision: <what was chosen>`
6. `## Alternatives considered`
7. `## Consequences: <what changes, what becomes harder>`
8. `## Supersession history` (if applicable)

**Generic examples:** `decision-payment-provider-selection`; `decision-outbox-pattern-over-message-broker`; `decision-monolith-vs-microservices-mvp`.

**When to use:** A choice was made and needs to be traceable — who decided, when, with what information, and why.

**When not to use:** If still under evaluation, use `explanation`. If documenting how to execute the decision, use `how-to`.

---

### 2.3 `explanation` — Analysis and Strategic Context

**What it is:** Market analysis, pivot context, thematic research, comparative study, opportunity assessment.

**Required structure (in this order):**
1. Frontmatter
2. TL;DR
3. `## Context: <what motivated this analysis>`
4. `## Analysis: <main body, subdivided by theme>`
5. `## Implications: <what this means for the project>`
6. `## Open questions or next decisions` (if any)

**Generic examples:** Market research on solo service providers 2026; analysis of CLG vs. PLG pre-launch; competitive landscape for B2B SaaS in payments.

**When to use:** The content is interpretive, changes over time, and the goal is to build understanding — not record a decision or describe a process.

**When not to use:** If the conclusion became a formal decision, create a parallel `decision` doc. Do not embed the decision inside the `explanation`.

---

### 2.4 `how-to` — Step-by-Step Operational Process

**What it is:** Execution guide for a recurring task — customer onboarding, payment account setup, release deploy, support process.

**Required structure (in this order):**
1. Frontmatter
2. TL;DR
3. `## Prerequisites: <what is needed before starting>`
4. `## Steps: <numbered, one per line, imperative verb>`
5. `## Verification: <how to confirm it worked>`
6. `## Common problems and fixes` (optional but recommended)

**Generic examples:** How to onboard a new customer; how to run a database migration in production; how to cut a release and deploy to staging.

**When to use:** There is a sequence of steps a person needs to execute and order matters.

**When not to use:** If conceptual (why we do it this way), use `explanation`. If recording a decision to change the process, use `decision`.

---

### 2.5 `capture` — Raw Idea or Seed

**What it is:** Quick note, unshaped idea, unexplored hypothesis, seed that does not yet know what it will become.

**Required structure (in this order):**
1. Frontmatter (minimum: `type`, `status: draft`, `created`)
2. `## Idea: <what it is in one sentence>`
3. `## Why it might matter` (free-form, no rigid structure)
4. `## Suggested next step` (optional)

**Generic examples:** `idea-anti-fraud-webhook-monitor`; seed for WhatsApp Business API integration; pricing hypothesis — per-seat vs. flat monthly.

**When to use:** Something surfaced and needs to be captured now, before there is time to classify it. The doc migrates to `reference`, `explanation`, or `decision` once it matures.

**When not to use:** If there is already enough structure and evidence for an `explanation`, go directly to the right type.

---

## Section 3 — Required Frontmatter: Full Specification

### 3.1 YAML Frontmatter (Git)

```yaml
---
type: reference | decision | explanation | how-to | capture
status: draft | active | review | superseded | archived
# For type: decision, use: proposed | accepted | deprecated | superseded
owner: "@handle"
tags:
  - kebab-case
  - icp-solo-founder
  - payment-provider
  - infra-decision
related:
  - docs/specs/00001-outbox-pattern.md
  - https://your-confluence-instance/pages/185565259
created: YYYY-MM-DD
updated: YYYY-MM-DD
# Additional fields for type: decision
supersedes: ""           # slug of the prior doc (empty if not applicable)
superseded-by: ""        # filled when this doc is superseded
review-date: ""          # optional target date for re-evaluation
---
```

### 3.2 Page Properties Macro (Confluence — XML Storage Format)

```xml
<ac:structured-macro ac:name="details">
  <ac:rich-text-body>
    <table>
      <tbody>
        <tr><th>type</th><td>reference</td></tr>
        <tr><th>status</th><td>active</td></tr>
        <tr><th>owner</th><td>@handle</td></tr>
        <tr><th>tags</th><td>icp-solo-founder, saas</td></tr>
        <tr><th>related</th>
          <td>
            <ac:link><ri:page ri:content-title="Editorial Standards"/></ac:link>
          </td>
        </tr>
        <tr><th>created</th><td>YYYY-MM-DD</td></tr>
        <tr><th>updated</th><td>YYYY-MM-DD</td></tr>
      </tbody>
    </table>
  </ac:rich-text-body>
</ac:structured-macro>
```

Fields `supersedes`, `superseded-by`, and `review-date` must be added as extra rows when `type` is `decision`.

---

## Section 4 — Page Structure: Progressive Disclosure

Every page follows this order. No exceptions.

### Fixed order

1. **Frontmatter** — Page Properties macro (Confluence) or YAML (Git). Always the first element.
2. **TL;DR** — 5 bullets max; required on pages > 300 words. Use the `excerpt` macro on Confluence so search indexes the summary as a snippet.
3. **Context header** — Always qualified: `## Context: why we moved from a message broker to Outbox on Postgres`. Never bare `## Context`.
4. **Body** — Type-specific sections (see Section 2). Self-contained paragraphs.
5. **Appendix** (optional) — Inside an `expand` macro (Confluence) or `<details>` block (Markdown). Raw data, entity version history, screenshots, call transcripts.

### Chunking-friendly writing rules

These rules ensure an AI agent can answer from an isolated chunk without needing the full document. They are based on retrieval benchmarks showing a 30–40% precision drop when chunks depend on context from earlier sections.

- **Unique, descriptive headers.** The path `Section > Subsection` must uniquely identify the content. `Analysis > Context` is useless. `Market Research 2026 > Context: demand saturation by region` is retrievable.
- **Paragraphs of 3–5 sentences, one idea per paragraph.** No "as seen above" or "as mentioned earlier."
- **Complete tables within a single section.** Never split a table across sections or pages — the model loses the header-to-row relationship.
- **No relative cross-references.** Never "see the previous point" or "in the next section." Reference by full section title or explicit link.

---

## Section 5 — Naming Convention: Slugs and Page Titles

### Slug pattern

```
{type}-{entity}-{context}
```

| Internal slug | Visible title |
|---|---|
| `competitor-stripe` | Competitor: Stripe |
| `decision-payment-provider-selection` | Decision: Payment Provider Selection |
| `research-market-solo-service-providers-2026` | Research: Market — Solo Service Providers (2026) |
| `process-customer-onboarding` | Process: Customer Onboarding |
| `idea-anti-fraud-webhook-monitor` | Idea: Anti-Fraud Webhook Monitor |
| `icp-solo-saas-founder` | ICP: Solo SaaS Founder |

### Rules

- Slug: lowercase, kebab-case, no accents, no spaces, no special characters (`@`, `.`, `/`).
- Visible title (Confluence page title or `# H1` in Git): may use capitalization, colons, and punctuation.
- The slug is used as the internal page identifier, Git filename, and value in `related` fields.
- The type in the slug must match the `type` field in the frontmatter exactly.

---

## Section 6 — Cross-Linking: No Page Is Born Alone

### Three non-negotiable requirements

1. **Every page MUST be listed in the project KNOWLEDGE_MAP.** The KNOWLEDGE_MAP is the central index table; without it, search and agents are unaware the page exists.
2. **Every page MUST have at least 1 incoming link** (another page that references it). A page with no incoming links is an orphan — prohibited.
3. **Every page MUST have the `related` field filled** with ≥ 1 link, or explicitly declared `related: []` with an inline comment justifying the absence (acceptable only for very early `capture` docs).

### When to use each ADF link type

| Situation | Link type |
|---|---|
| Quick inline reference where destination is obvious | Plain hyperlink |
| Reference in a table, list, or mid-sentence where context helps | `inlineCard` (ADF) |
| Status card — Jira, PR, Linear — in a dedicated section | `blockCard` (ADF) |
| Visual content that saves a click — Figma, Miro, Loom, video | `embedCard` (ADF) |

The skill generates the correct ADF for each variant. Do not use `blockCard` for inline text links — the visual card interrupts reading flow.

---

## Section 7 — Decision Immutability: The Hard Rule

Any doc with `type: decision` and `status: accepted` is immutable in content. This means:

- Do not fix relevant body errors (obvious typographic fixes only are tolerated).
- Do not add "context that was missing."
- Do not change the conclusion.

**Process for changing a decision:**

1. Create a new `decision` doc with `supersedes: <slug-of-old>` in frontmatter.
2. On the old doc: change `status` to `superseded`, add `superseded-by: <slug-of-new>`.
3. Do not delete the old doc. The history is the asset.

This applies to both Confluence (rare for decisions) and Git under `/docs/specs/` (where ADRs and specs live).

**Example:** The decision to select a payment provider at launch (`decision-payment-provider-selection`, status `accepted`) is not edited when switching providers later. Create `decision-payment-provider-v2` with `supersedes: decision-payment-provider-selection`, then update the old doc to `superseded`.

---

## Section 8 — No Process Noise in the Doc Body

Pages do not document their own editing history. The body of a page is about the subject of the page — not about how the doc was written, reviewed, or corrected.

**Prohibited in the body:**
- "Rewritten after May review"
- "v1 used approach X; this version switches to Y"
- "Corrected error identified by @handle on 10/05"
- Sections named "Changelog", "Version history", "What changed"

**Where history goes:**
- Creation date and last update: in `created` / `updated` frontmatter fields.
- Change of decision: via supersession (Section 7), not inline comment.
- Context for why the doc exists: in the `## Context` section of the doc itself, written as present fact, not as an editing narrative.

The only exception: `type: decision` docs that explicitly include `## Supersession history` — but this documents the chain of decisions, not the editing of the file.

---

## Section 9 — Anti-Patterns: What Never to Do

| Anti-pattern | Problem | Fix |
|---|---|---|
| Orphan page (no incoming link) | Invisible to agents and search | Add to KNOWLEDGE_MAP + link from at least 1 related page |
| Missing frontmatter | Agent cannot determine type, status, or owner | Add Page Properties macro before publishing |
| Missing TL;DR on page > 300 words | Reader (human or AI) must read everything to grasp the point | Add `excerpt` block with ≤ 5 bullets at the top |
| Generic header (`## Context`, `## Analysis`) | Chunk has no identity — unrecoverable by retrieval | Always qualify: `## Analysis: demand seasonality for subscription products` |
| Wall of text (paragraph > 8 lines without a break) | Degrades both human reading and AI chunking | Break into 3–5 sentence paragraphs or use a list |
| Narrow table column layout | Illegible; Confluence breaks the rendering | Use full-width layout or place table in a 2-column section |
| Editing `decision` with `status: accepted` | Erases historical context; change becomes unauditable | Supersede with a new doc (Section 7) |
| Creating a page without running `confluence-docs check` | May duplicate existing content | Always run `check` before creating |
| Duplicating content between pages | Two sources of truth diverge inevitably | Use `excerpt-include` macro to reference, not copy |

---

## Section 10 — How the Skill Consumes This File

The skill reads `reference/doc-types.md` (this file) as its canonical contract for all page creation and validation operations. No template logic is hardcoded in the skill — all behavior derives from Sections 2, 3, and 4 here. `SKILL.md` and the `km` command derive their defaults and validation rules from these sections.

**Subcommands:**

- `new <type>` — Generates an empty template with complete frontmatter and the required section structure for the specified type (Section 2). The template includes required headers in the correct order, with descriptive placeholders.
- `lint <page-id or path>` — Checks conformance: frontmatter present, TL;DR if > 300 words, generic headers flagged, `related` field populated, valid slug format.
- `check` — Queries the KNOWLEDGE_MAP before creating a new page. If a similar title or slug already exists, alerts with a link to the existing doc before proceeding.

When this file is updated (e.g., a new type added, a frontmatter field revised), the skill inherits the new behavior on the next run — no separate skill update required.
