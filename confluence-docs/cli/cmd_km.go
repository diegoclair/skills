// cmd_km.go — `confluence-docs km` subcommand.
//
// Generates and optionally uploads the Lybel KNOWLEDGE_MAP page, consolidating
// pages from a triage directory (batch-*.json from subagent classification) and
// an optional baseline JSON file (the original hand-classified pages).
//
// Usage:
//
//	confluence-docs km generate \
//	    --input /tmp/lybel-triage \
//	    --baseline baseline.json \
//	    --target-page-id 200441858 \
//	    --message "regenerate KM"
//
//	confluence-docs km classify --page-id 12345   # stub — not implemented
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lybel-app/skills/confluence-docs/cli/adf"
)

// ── Data types ─────────────────────────────────────────────────────────────

// triageEntry is one row produced by a subagent triage batch.
type triageEntry struct {
	PageID        string   `json:"pageId"`
	Title         string   `json:"title"`
	TipoProposto  string   `json:"tipo_proposto"`
	Confidence    string   `json:"confidence,omitempty"`
	TagsSugeridas []string `json:"tags_sugeridas,omitempty"`
	Rationale     string   `json:"rationale,omitempty"`
	Anomalia      *string  `json:"anomalia"`
}

// baselineEntry is one row in the hand-classified baseline file.
type baselineEntry struct {
	PageID string   `json:"pageId"`
	Title  string   `json:"title"`
	Tipo   string   `json:"tipo"`
	Tags   []string `json:"tags"`
}

// baseline is the top-level structure of the baseline JSON file.
type baseline struct {
	Pages []baselineEntry `json:"pages"`
}

// kmPage is the unified representation used for rendering.
type kmPage struct {
	ID          string
	Title       string
	Tipo        string
	Tags        []string
	FaseTag     string  // "fase-final-checkout-universal" if applicable (kept in Tags)
	RealAnomaly string  // empty when none
	Confidence  string
}

// ── Main router ───────────────────────────────────────────────────────────

// runKM handles `confluence-docs km`.
func runKM(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		printKMHelp(stdout)
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "generate":
		return runKMGenerate(args[1:], stdout, stderr)
	case "classify":
		return runKMClassify(args[1:], stdout, stderr)
	case "-h", "--help":
		printKMHelp(stdout)
		return exitOK, nil
	default:
		fmt.Fprintln(stderr, "km: unknown subcommand:", args[0])
		fmt.Fprintln(stderr, "  valid subcommands: generate, classify")
		return exitInputErr, errInvalidUsage
	}
}

func printKMHelp(w io.Writer) {
	fmt.Fprintln(w, `km — generate and optionally upload the Lybel KNOWLEDGE_MAP page.

SUBCOMMANDS:
  generate    Consolidate triage batches + baseline into markdown, and optionally upload.
  classify    Stub: classify a single page (not implemented yet).

USAGE (generate):
  confluence-docs km generate \
      [--input DIR]             triage directory with batch-*.json files (default: /tmp/lybel-triage)
      [--baseline FILE]         baseline JSON with hand-classified pages
      [--target-page-id ID]     if set, upload result to this Confluence page
      [--output FILE]           write markdown to FILE (default: stdout when no --target-page-id)
      [--dry-run]               render without uploading
      [--message "..."]         version comment for upload (default: "regenerate KM")
      [--full-width]            set page to full-width after upload

BASELINE FORMAT (--baseline FILE):
  {
    "pages": [
      {"pageId": "185303042", "title": "Sobre a Lybel", "tipo": "reference", "tags": []},
      {"pageId": "187695141", "title": "Proposta fit HOJE",  "tipo": "decision", "tags": ["fase-mvp"]}
    ]
  }

EXIT CODES:
  0  success
  2  invalid flags / missing files
  3  upload error`)
}

// ── Generate ──────────────────────────────────────────────────────────────

func runKMGenerate(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		inputDir     = "/tmp/lybel-triage"
		baselineFile string
		targetPageID string
		outputFile   string
		message      = "regenerate KM"
		dryRun       bool
		fullWidth    bool
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--input":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--input requires a directory path")
				return exitInputErr, errInvalidUsage
			}
			inputDir = remaining[i+1]
			i++
		case "--baseline":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--baseline requires a file path")
				return exitInputErr, errInvalidUsage
			}
			baselineFile = remaining[i+1]
			i++
		case "--target-page-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--target-page-id requires a page ID")
				return exitInputErr, errInvalidUsage
			}
			targetPageID = remaining[i+1]
			i++
		case "--output":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--output requires a file path")
				return exitInputErr, errInvalidUsage
			}
			outputFile = remaining[i+1]
			i++
		case "--message":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--message requires a value")
				return exitInputErr, errInvalidUsage
			}
			message = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "--full-width":
			fullWidth = true
		case "-h", "--help":
			printKMHelp(stdout)
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	// Load triage entries.
	triageEntries, err := loadTriageDir(inputDir)
	if err != nil {
		fmt.Fprintln(stderr, "km generate: loading triage:", err)
		return exitInputErr, err
	}

	// Load baseline (optional).
	var bl baseline
	if baselineFile != "" {
		bl, err = loadBaseline(baselineFile)
		if err != nil {
			fmt.Fprintln(stderr, "km generate: loading baseline:", err)
			return exitInputErr, err
		}
	}

	// Merge and classify.
	pageMap := mergePages(bl, triageEntries)

	// Render markdown.
	md := renderKMMD(pageMap)

	// Output.
	if targetPageID != "" && !dryRun {
		// Upload to Confluence.
		client, ok := buildClient(cloud, email, token, stderr)
		if !ok {
			return exitUnknownErr, nil
		}
		if err := uploadKM(client, targetPageID, md, fullWidth, message, stderr); err != nil {
			fmt.Fprintln(stderr, "km generate: upload failed:", err)
			return exitUnknownErr, err
		}
		fmt.Fprintf(stdout, `{"status":"ok","pageId":%q}`+"\n", targetPageID)
	} else {
		// Write to file or stdout.
		if dryRun && targetPageID != "" {
			fmt.Fprintf(stderr, "dry-run: would upload %d bytes to page %s\n", len(md), targetPageID)
		}
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(md), 0644); err != nil {
				fmt.Fprintln(stderr, "writing output:", err)
				return exitUnknownErr, err
			}
			fmt.Fprintf(stderr, "wrote %d bytes to %s\n", len(md), outputFile)
		} else {
			fmt.Fprint(stdout, md)
		}
	}
	return exitOK, nil
}

// ── Classify (stub) ───────────────────────────────────────────────────────

func runKMClassify(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--page-id":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			pageID = args[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "classify --page-id ID  [not implemented]")
			fmt.Fprintln(stdout, "  Stub: returns {tipo, tags_sugeridas, ...} for a given page.")
			return exitOK, nil
		}
	}
	_ = pageID
	fmt.Fprintln(stderr, "km classify: not implemented")
	return exitUnknownErr, fmt.Errorf("not implemented")
}

// ── I/O helpers ───────────────────────────────────────────────────────────

// loadTriageDir reads all batch-*.json files from dir and returns the merged
// list of triage entries. Returns an empty slice (not an error) if the dir
// exists but has no matching files.
func loadTriageDir(dir string) ([]triageEntry, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "batch-*.json"))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	sort.Strings(matches)

	var out []triageEntry
	for _, path := range matches {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("reading %s: %w", path, readErr)
		}
		var batch []triageEntry
		if jsonErr := json.Unmarshal(data, &batch); jsonErr != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, jsonErr)
		}
		out = append(out, batch...)
	}
	return out, nil
}

// loadBaseline parses the baseline JSON file.
func loadBaseline(path string) (baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return baseline{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var bl baseline
	if err := json.Unmarshal(data, &bl); err != nil {
		return baseline{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return bl, nil
}

// ── Merge & classify ──────────────────────────────────────────────────────

// faseFinalAnomaliaSubstrings are anomaly-field substrings that signal the
// "Fase Final — Checkout Universal" horizon (more specific than the tag list,
// matching the Python is_fase_final_marker heuristic).
var faseFinalAnomaliaSubstrings = []string{
	"obsoleto-pos-pivot", "conteudo-desatualizado", "pre-pivot",
	"pos-pivot", "b2b2c-legacy", "b2b2c", "checkout b2b2c", "conteudo-raso-pre-pivot",
}

// realAnomalySubstrings are anomaly substrings that indicate a genuine issue
// requiring human review.
var realAnomalySubstrings = []string{
	"borderline", "duplicata", "nome-desatualizado",
}

// pejorativeTagSubstrings are tag substrings considered pejorative and
// replaced by the canonical "fase-final-checkout-universal" tag.
var pejorativeTagSubstrings = []string{
	"legacy", "obsoleto", "desatualizad", "pre-pivot", "pos-pivot", "antigo",
}

// classifyFase analyses the raw anomaly string and raw tags, returning:
//   - addFaseTag: true if "fase-final-checkout-universal" should be added
//   - cleanedTags: input tags with pejorative substrings removed and canonical tag added if needed
//   - realAnomaly: the anomaly string if it is a real anomaly, else ""
func classifyFase(anomalia string, rawTags []string) (addFaseTag bool, cleanedTags []string, realAnomaly string) {
	aLower := strings.ToLower(anomalia)

	// Detect horizon markers in anomalia.
	for _, kw := range faseFinalAnomaliaSubstrings {
		if strings.Contains(aLower, kw) {
			addFaseTag = true
			break
		}
	}

	// Detect real anomaly.
	for _, kw := range realAnomalySubstrings {
		if strings.Contains(aLower, kw) {
			realAnomaly = anomalia
			break
		}
	}

	// Clean tags: replace pejorative tags with canonical one.
	hasFaseCanonical := false
	for _, t := range rawTags {
		tl := strings.ToLower(t)
		isPejorative := false
		for _, p := range pejorativeTagSubstrings {
			if strings.Contains(tl, p) {
				isPejorative = true
				addFaseTag = true
				break
			}
		}
		if isPejorative {
			continue
		}
		if tl == "fase-final-checkout-universal" {
			hasFaseCanonical = true
			addFaseTag = true
		}
		cleanedTags = append(cleanedTags, t)
	}

	// Inject canonical tag once if warranted.
	if addFaseTag && !hasFaseCanonical {
		cleanedTags = append(cleanedTags, "fase-final-checkout-universal")
	}
	return addFaseTag, cleanedTags, realAnomaly
}

// mergePages builds the unified page map from baseline (higher precedence) and
// triage entries. Returns map[pageID]kmPage.
func mergePages(bl baseline, triage []triageEntry) map[string]kmPage {
	pages := make(map[string]kmPage)

	// Seed from baseline (highest confidence).
	for _, b := range bl.Pages {
		pages[b.PageID] = kmPage{
			ID:    b.PageID,
			Title: b.Title,
			Tipo:  b.Tipo,
			Tags:  append([]string(nil), b.Tags...),
		}
	}

	// Overlay triage entries.
	for _, e := range triage {
		pid := strings.TrimSpace(e.PageID)
		if pid == "" {
			continue
		}
		anomaliaStr := ""
		if e.Anomalia != nil {
			anomaliaStr = *e.Anomalia
		}
		addFase, cleanedTags, realAnomaly := classifyFase(anomaliaStr, e.TagsSugeridas)

		if existing, ok := pages[pid]; ok {
			// Baseline entry already exists: only augment, never override tipo/title.
			if addFase {
				hasFase := false
				for _, t := range existing.Tags {
					if t == "fase-final-checkout-universal" {
						hasFase = true
						break
					}
				}
				if !hasFase {
					existing.Tags = append(existing.Tags, "fase-final-checkout-universal")
				}
			}
			if realAnomaly != "" && existing.RealAnomaly == "" {
				existing.RealAnomaly = realAnomaly
			}
			pages[pid] = existing
		} else {
			pages[pid] = kmPage{
				ID:          pid,
				Title:       e.Title,
				Tipo:        e.TipoProposto,
				Tags:        cleanedTags,
				RealAnomaly: realAnomaly,
				Confidence:  e.Confidence,
			}
		}
	}
	return pages
}

// ── Render ────────────────────────────────────────────────────────────────

const kmURLBase = "https://lybel.atlassian.net/wiki/spaces/lybel/pages/"

type tipoMeta struct {
	label string
	desc  string
}

var tipoOrder = []string{"reference", "decision", "explanation", "how-to", "capture"}

var tipoMetas = map[string]tipoMeta{
	"reference":   {"📚 reference", "Entidades estáveis: concorrentes, parceiros, advisors, ferramentas, ICPs, features."},
	"decision":    {"⚖️ decision", "Escolhas registradas: ADRs, pivots, princípios, modelos."},
	"explanation": {"🔍 explanation", "Análises e contexto: pesquisas, growth, cases, estudos comparativos."},
	"how-to":      {"🛠️ how-to", "Processos operacionais passo-a-passo."},
	"capture":     {"💡 capture", "Ideias e hipóteses em validação."},
}

// renderKMMD produces the full markdown body for the KNOWLEDGE_MAP page.
func renderKMMD(pages map[string]kmPage) string {
	// Group by tipo.
	byTipo := make(map[string][]kmPage)
	for _, p := range pages {
		byTipo[p.Tipo] = append(byTipo[p.Tipo], p)
	}
	for tipo := range byTipo {
		sort.Slice(byTipo[tipo], func(i, j int) bool {
			return strings.ToLower(byTipo[tipo][i].Title) < strings.ToLower(byTipo[tipo][j].Title)
		})
	}

	// Count totals.
	totals := make(map[string]int)
	for _, t := range tipoOrder {
		totals[t] = len(byTipo[t])
	}
	totalAll := 0
	for _, n := range totals {
		totalAll += n
	}
	faseFinalCount := 0
	for _, p := range pages {
		for _, tag := range p.Tags {
			if tag == "fase-final-checkout-universal" {
				faseFinalCount++
				break
			}
		}
	}

	today := time.Now().Format("2006-01-02")

	var sb strings.Builder
	wl := func(s string) { sb.WriteString(s); sb.WriteByte('\n') }

	// Properties frontmatter.
	// `collapsed` wraps the details macro in an expand — the metadata table is
	// big and noisy at the top of a 122-page index. Click to see.
	wl(":::properties collapsed")
	wl("tipo: reference")
	wl("status: ativo")
	wl("owner: @diegoclair")
	wl("tags: meta-doc, ai-first, tipo-index, knowledge-management")
	wl("relacionados: [[Home]], docs/standards/EDITORIAL_v2.md")
	wl("criado: " + today)
	wl("atualizado: " + today)
	wl(":::")
	wl("")

	// TL;DR.
	wl("## TL;DR")
	wl("")
	wl("- Mapa cross-topic do conhecimento Lybel organizado por **tipo de doc** (não por área). Agentes IA leem antes de criar página nova.")
	wl(fmt.Sprintf(
		"- %d páginas classificadas em 5 tipos: `reference` (%d), `explanation` (%d), `capture` (%d), `decision` (%d), `how-to` (%d).",
		totalAll, totals["reference"], totals["explanation"], totals["capture"], totals["decision"], totals["how-to"],
	))
	wl("- Tag `fase-mvp` = páginas do Cenário I (caminho/MVP atual). Tag `fase-final-checkout-universal` = páginas do produto-fim (B2B2C, visão de longo prazo) — **preservadas como contexto fundacional, NÃO obsoletas**.")
	wl(fmt.Sprintf(
		"- Hoje **%d páginas** carregam tag `fase-final-checkout-universal`. Sem juízo de valor — só filtro pra você encontrar quando quiser revisitar a visão.",
		faseFinalCount,
	))
	wl("- Para navegação por área temática (Estratégia/Produto/Research), use a [Home](https://lybel.atlassian.net/wiki/spaces/lybel/overview). Os dois mapas são complementares.")
	wl("")

	// Por que este mapa existe.
	wl("## Por que este mapa existe")
	wl("")
	wl("A Home organiza por **área temática** (9 categorias). O KNOWLEDGE_MAP organiza por **tipo de doc** — eixo cross-cutting que IA usa pra decidir tom, estrutura e regras de imutabilidade.")
	wl("")
	wl("Spec canônica completa dos 5 tipos: `docs/standards/EDITORIAL_v2.md` no Git.")
	wl("")

	// Os 5 tipos table.
	wl("## Os 5 tipos")
	wl("")
	wl("| Tipo | O que é | Onde vive | Imutável? |")
	wl("|---|---|---|---|")
	wl("| `reference` | Fato estável sobre entidade externa/interna | Confluence | Não |")
	wl("| `decision` | Escolha registrada com contexto e consequências | Git `/docs/specs/` preferencial | **Sim** após `accepted` |")
	wl("| `explanation` | Análise, contexto estratégico, pesquisa | Confluence | Não |")
	wl("| `how-to` | Processo operacional passo-a-passo | Confluence | Não |")
	wl("| `capture` | Ideia bruta, hipótese em validação | Confluence | Não — migra quando madura |")
	wl("")
	wl("---")
	wl("")

	// Regras pra IA — H2 PRÓPRIO antes das seções de tipo, pra digest enxergar.
	wl("## Regras pra IA (LEIA ANTES DE CRIAR PÁGINA NOVA)")
	wl("")
	wl(":::info Sequência obrigatória")
	wl("")
	wl("1. **INDEX-FIRST**: rode `confluence-docs check --title \"...\" --type <tipo>` antes de criar. Se já existe similar (score ≥ 0.4 default), atualize-a.")
	wl("2. **Fallback semântico**: se `check` retornar vazio, rode `confluence-docs search \"<termo curto>\"` antes de criar — trigrama pega só similaridade lexical, search pega texto + título e cobre o gap.")
	wl("3. **Use `confluence-docs new <tipo>`** pra gerar template padrão pros 5 tipos (frontmatter + estrutura).")
	wl("4. **Registre aqui no mesmo turno**: toda página nova entra na seção do tipo correspondente neste mapa, no mesmo turno em que é criada.")
	wl("5. **Inserção no KM**: use `confluence-docs page apply --insert-after \"## 📚 reference\"` (ou tipo correspondente). O bullet segue formato: `- [Title](URL) (pageId) — tags: \\`t1\\`, \\`t2\\``.")
	wl("6. **Slug** segue padrão `{tipo}-{entidade}-{contexto}` (kebab-case, sem acento).")
	wl("7. **Frontmatter** via `:::properties` é obrigatório (Page Properties macro real desde v0.8.0).")
	wl("8. **TL;DR ≤ 5 bullets** obrigatório se página > 300 palavras.")
	wl("9. **Headers descritivos**: `## Contexto: <qualificador>`, nunca só `## Contexto`.")
	wl("10. **Decisões superseded**: ao criar nova decision com `supersedes: <id-antigo>`, ATUALIZE o doc antigo: `status: superseded`, adicione `superseded-by: <id-novo>`. Nunca apaga.")
	wl("11. **Tag de horizonte é opcional mas recomendada**: `fase-mvp` (Cenário I, MVP atual) ou `fase-final-checkout-universal` (produto-fim, visão de longo prazo). Sem juízo de valor — só filtro.")
	wl("12. **Tag de fase ausente é válida**: páginas cross-cutting (índices, frameworks, marca) podem ficar sem tag de horizonte.")
	wl("13. **Spec canônica completa**: `docs/standards/EDITORIAL_v2.md` no Git.")
	wl("")
	wl(":::")
	wl("")
	wl("---")
	wl("")

	// Sections per tipo. Headings WITHOUT counts — `page apply --insert-after "## 📚 reference"` deve ser estável.
	for _, tipo := range tipoOrder {
		meta := tipoMetas[tipo]
		items := byTipo[tipo]
		wl(fmt.Sprintf("## %s", meta.label))
		wl("")
		wl(fmt.Sprintf("_%d páginas — %s_", len(items), meta.desc))
		wl("")
		if len(items) == 0 {
			wl("_Nenhuma página classificada._")
			wl("")
			wl("---")
			wl("")
			continue
		}
		if len(items) > 12 {
			wl(fmt.Sprintf(":::expand Ver as %d páginas", len(items)))
			for _, p := range items {
				wl(renderKMPageLine(p))
			}
			wl(":::")
		} else {
			for _, p := range items {
				wl(renderKMPageLine(p))
			}
		}
		wl("")
		wl("---")
		wl("")
	}

	// Anomalias section.
	wl("## Anomalias e casos pra revisão humana")
	wl("")
	wl("Só anomalias **reais** entram aqui: fronteira ambígua entre tipos (`borderline-tipo`), suspeita de duplicata, nome desatualizado. **Tag `fase-final-checkout-universal` NÃO é anomalia** — é só horizonte.")
	wl("")
	var anomalias []kmPage
	for _, p := range pages {
		if p.RealAnomaly != "" {
			anomalias = append(anomalias, p)
		}
	}
	sort.Slice(anomalias, func(i, j int) bool {
		return strings.ToLower(anomalias[i].Title) < strings.ToLower(anomalias[j].Title)
	})
	if len(anomalias) > 0 {
		wl(fmt.Sprintf(":::expand %d páginas com anomalia real pra revisão", len(anomalias)))
		for _, p := range anomalias {
			a := p.RealAnomaly
			if len(a) > 90 {
				// Truncate at first colon or em-dash.
				if idx := strings.Index(a, ":"); idx > 0 && idx < 90 {
					a = a[:idx]
				} else if idx := strings.Index(a, " — "); idx > 0 && idx < 90 {
					a = a[:idx]
				} else {
					a = a[:90] + "…"
				}
			}
			wl(fmt.Sprintf("- [%s](%s%s) `(%s)` — tipo: `%s`", p.Title, kmURLBase, p.ID, p.ID, p.Tipo))
			wl(fmt.Sprintf("  - %s", a))
		}
		wl(":::")
	} else {
		wl("_Nenhuma anomalia real flagada._")
	}
	wl("")

	// Ver também.
	wl("## Ver também")
	wl("")
	wl("- [Home — Navegação por área temática](https://lybel.atlassian.net/wiki/spaces/lybel/overview)")
	wl("- `docs/standards/EDITORIAL_v2.md` (Git) — spec canônica completa dos 5 tipos, frontmatter, anti-padrões")
	wl("")

	// Manutenção.
	wl("## Manutenção")
	wl("")
	wl("- Toda página nova é adicionada à seção do tipo correspondente no mesmo turno em que é criada.")
	wl("- Reclassificações (`capture` → `explanation`, etc) também atualizam aqui.")
	wl(fmt.Sprintf("- A classificação atual cobre as %d páginas do espaço (%s). Páginas adicionais entram à medida que são criadas.", totalAll, today))

	return sb.String()
}

// renderKMPageLine formats a single page entry in the KNOWLEDGE_MAP lists.
func renderKMPageLine(p kmPage) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- [%s](%s%s) `(%s)`", p.Title, kmURLBase, p.ID, p.ID))
	if len(p.Tags) > 0 {
		sb.WriteString(" — tags: ")
		for i, t := range p.Tags {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("`")
			sb.WriteString(t)
			sb.WriteString("`")
		}
	}
	if p.RealAnomaly != "" {
		a := p.RealAnomaly
		if len(a) > 90 {
			if idx := strings.Index(a, ":"); idx > 0 && idx < 90 {
				a = a[:idx]
			} else if idx := strings.Index(a, " — "); idx > 0 && idx < 90 {
				a = a[:idx]
			} else {
				a = a[:90] + "…"
			}
		}
		sb.WriteString(" ⚠️ ")
		sb.WriteString(a)
	}
	return sb.String()
}

// ── Upload ────────────────────────────────────────────────────────────────

// uploadKM converts markdown to storage format (because the KM uses :::properties
// and :::info / :::expand blocks) and uploads it to Confluence. It also
// extracts the `tags:` line from the :::properties block and applies them as
// real Confluence labels on the page (so they appear as clickable chips above
// the title, not only as table cell text).
func uploadKM(client *adf.ConfluenceClient, pageID, md string, fullWidth bool, message string, stderr io.Writer) error {
	// KM markdown always contains :::properties and :::info — storage path.
	storageBody, err := adf.MarkdownToStorage([]byte(md))
	if err != nil {
		return fmt.Errorf("convert markdown to storage: %w", err)
	}
	if err := client.UpdatePageStorage(pageID, "", 0, storageBody, message, false, stderr); err != nil {
		return err
	}
	if fullWidth {
		if err := client.SetPageAppearance(pageID, adf.PageAppearanceFullWidth); err != nil {
			fmt.Fprintf(stderr, "warning: page updated but full-width could not be set: %v\n", err)
		}
	}
	// Apply tags from the :::properties block as real Confluence labels.
	labels := extractTagsFromProperties(md)
	if len(labels) > 0 {
		if err := client.AddLabels(pageID, labels); err != nil {
			fmt.Fprintf(stderr, "warning: page updated but labels could not be applied: %v\n", err)
		}
	}
	return nil
}

// extractTagsFromProperties scans the markdown for a :::properties block and
// returns the comma-separated tag list as individual normalized labels (lower-
// case, kebab-case). Returns nil if no :::properties block or no tags line.
func extractTagsFromProperties(md string) []string {
	lines := strings.Split(md, "\n")
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if strings.HasPrefix(trimmed, ":::") && strings.Contains(trimmed, "properties") {
				inBlock = true
			}
			continue
		}
		if trimmed == ":::" {
			break
		}
		// Look for "tags: a, b, c" (case-insensitive key).
		if idx := strings.Index(trimmed, ":"); idx > 0 {
			key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
			if key == "tags" {
				val := strings.TrimSpace(trimmed[idx+1:])
				if val == "" {
					return nil
				}
				parts := strings.Split(val, ",")
				out := make([]string, 0, len(parts))
				for _, p := range parts {
					p = strings.TrimSpace(p)
					// Confluence labels are alphanumeric + dash; replace spaces/underscores.
					p = strings.ReplaceAll(p, " ", "-")
					p = strings.ReplaceAll(p, "_", "-")
					p = strings.ToLower(p)
					if p != "" {
						out = append(out, p)
					}
				}
				return out
			}
		}
	}
	return nil
}
