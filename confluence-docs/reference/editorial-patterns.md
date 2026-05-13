# Editorial Patterns — How Confluence pages should be written

> **When to read:** before creating or substantially editing any **decision**, **proposal**, **strategy**, or **spec** page. Index pages, glossaries and pure reference docs can skip the Context→Problem→Solution structure but still benefit from Pattern 1 (header) and Pattern 3 (clarity for outside readers).

## Contents

- [Pattern 1 — Page header](#pattern-1--page-header)
- [Pattern 2 — Context → Problem → Solution structure](#pattern-2--context--problem--solution-structure)
- [Pattern 3 — Clarity for outside readers](#pattern-3--clarity-for-outside-readers)
- [Pattern 4 — No process meta-noise](#pattern-4--no-process-meta-noise-pretend-prior-versions-never-existed)

---

## Pattern 1 — Page header

Start every page with a small header block (quote/callout) containing:

> **Context:** (one-line summary of what the page is about)
> **Created:** YYYY-MM-DD | **Updated:** YYYY-MM-DD | **Author:** [name]

When updating a page, bump the "Updated" date. On first creation, author comes from the current user (confirm if unknown).

## Pattern 2 — Context → Problem → Solution structure

For decision / proposal / strategy pages, organize the body around three sections:

1. **Context** — where the page comes from, which project / moment it serves, what bigger scope it sits in
2. **Problem** — what is being solved, what hurts, what constraints exist
3. **Solution (proposed)** — proposed approach, options, tradeoffs, concrete scope

See `doc-types.md` § Section 2 (Decision type) for the canonical structure.

**Why this matters:** the knowledge base becomes predictable — any reader (human or AI) knows where to find the motivation, the pain, and the plan without re-reading everything. Reduces onboarding cost and review time.

**When creating a page**: apply these patterns by default; deviate only if the page type is clearly an index/reference/glossary.

**When updating an existing page** that violates the patterns: offer the user a refactor alongside the content change — don't silently rewrite structure.

## Pattern 3 — Clarity for outside readers

Pages are read by people who weren't in the conversation that generated them. Before saving:

- **Technical jargon** (Merchant of Record, Variant A, MCP, commission fee, postback, etc.) must be explained briefly on first use OR linked to a page that explains. Don't assume context.
- **Internal labels** (Variant A, Wave 2, v0.1) should come with a short gloss when first introduced on a page.
- **References to other work** (research, analysis, previous decisions) must be linked to the actual Confluence page — never just mention "we analyzed X" without linking X.
- **Balance**: don't over-explain the obvious (readers are smart, just not context-equipped). One short clause or parenthetical is usually enough. Prefer linking over inlining when the full explanation exists elsewhere.

**Smell test**: if a new team member opens this page cold, will they understand it? If not, add one link or one half-sentence.

## Pattern 4 — No process meta-noise. Pretend prior versions never existed.

The page is the current state of thinking — not a record of how it got there. Avoid documenting your own editing process inside the page body. Specifically, **never write**:

- ❌ "Replaced the previous version that had [problem]" / "This version supersedes the previous" / "Rewritten after re-research"
- ❌ "Was X, now is Y" / "Changed from A to B" (in the body of the page, as if the reader cared about the diff)
- ❌ "v1 → v2" comparisons or any version numbering inside content
- ❌ Apologetic notes about previous errors: "the CEO name was previously wrong", "was written in future tense before"
- ❌ Refactoring announcements: "this page was refactored today", "was a sub-section, now its own page"

**Why this rule:** today's "v1 vs v2" becomes tomorrow's confusion when v3 arrives. A reader 6 months from now doesn't care that the wrong name was once written here — they care if the right one is here now. **When fixing a factual error, just fix it. Pretend it never existed.** When restructuring, just restructure. Git history (or the page's own version history in Confluence) is the audit trail; the page body is the source of truth.

### History section — what to put, what NOT to put

When a page has a "History" / "Decision History" section, it records **substantive movements of the decision/state being documented** — not edits to the page itself. Use it for:

- ✅ A real strategic pivot ("Decision changed from X to Y after data Z came in") — write from the perspective of the *decision*, not the *page*.
- ✅ Regulatory or external events that shifted the page's conclusions ("Authority X published Resolution Y; we updated eligibility accordingly")
- ✅ A consolidated handoff moment ("Phase 0 closed on date X; this page archived its conclusions")

Do NOT use it for:

- ❌ "Refactored today, structure simplified" — that's an edit, not a movement
- ❌ "Morning version replaced by afternoon version" — same intra-day; the reader doesn't experience time as you do
- ❌ "Added section Y / removed section Z" — that's git history, not decision history
- ❌ Author or model self-references — "Sonnet researched", "AI agent updated"

**Smell test for History entries:** if you remove the entry, does the reader lose business context they'd otherwise have to ask about? If no, delete it. Many pages don't need a History section at all.

**Smell test for the rest of the body:** read the page as if you'd never seen it before. Anything that talks *about the page itself* (its versions, its corrections, its refactorings) is noise — strip it. The page is what it says, not how it got there.
