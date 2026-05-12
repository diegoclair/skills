// cmd_new.go — `confluence-docs new <tipo>` subcommand.
//
// Generates a markdown template for a new Confluence page of the given type,
// pre-filled with a :::properties block, TL;DR, and the structural headings
// appropriate for the doc type.
//
// Usage:
//
//	confluence-docs new reference --title "Análise Stripe Brasil" [--parent-id ID] [--full-width]
//	confluence-docs new decision  --title "..." [--supersedes ID]
//	confluence-docs new explanation --title "..."
//	confluence-docs new how-to    --title "..."
//	confluence-docs new capture   --title "..."
//
// The generated markdown is written to stdout (or --output FILE).
// Owner is resolved from git config user.email, falling back to $USER.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// docType is one of the five standard Lybel doc types.
type docType string

const (
	docTypeReference   docType = "reference"
	docTypeDecision    docType = "decision"
	docTypeExplanation docType = "explanation"
	docTypeHowTo       docType = "how-to"
	docTypeCapture     docType = "capture"
)

var validDocTypes = []docType{
	docTypeReference, docTypeDecision, docTypeExplanation, docTypeHowTo, docTypeCapture,
}

// runNew implements `confluence-docs new <tipo>`.
func runNew(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "new: requires a doc type: reference, decision, explanation, how-to, capture")
		fmt.Fprintln(stderr, "  Usage: confluence-docs new <type> --title \"...\" [--parent-id ID] [--full-width] [--output FILE]")
		return exitInputErr, errInvalidUsage
	}

	rawType := args[0]
	dt := docType(strings.ToLower(rawType))
	valid := false
	for _, v := range validDocTypes {
		if dt == v {
			valid = true
			break
		}
	}
	if !valid {
		fmt.Fprintf(stderr, "new: unknown doc type %q. Valid types: reference, decision, explanation, how-to, capture\n", rawType)
		return exitInputErr, errInvalidUsage
	}

	var (
		title      string
		parentID   string
		supersedes string
		outputFile string
		fullWidth  bool
	)

	for i := 1; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--title":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--title requires a value")
				return exitInputErr, errInvalidUsage
			}
			title = args[i+1]
			i++
		case "--parent-id":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--parent-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			parentID = args[i+1]
			i++
		case "--supersedes":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--supersedes requires a page ID")
				return exitInputErr, errInvalidUsage
			}
			supersedes = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--output requires a file path")
				return exitInputErr, errInvalidUsage
			}
			outputFile = args[i+1]
			i++
		case "--full-width":
			fullWidth = true
		case "-h", "--help":
			printNewHelp(stdout)
			return exitOK, nil
		default:
			// Accept title as positional for convenience
			if !strings.HasPrefix(a, "-") && title == "" {
				title = a
				continue
			}
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if title == "" {
		fmt.Fprintln(stderr, "new: --title is required")
		return exitInputErr, errInvalidUsage
	}

	today := time.Now().Format("2006-01-02")
	owner := resolveOwnerEmail()

	md := generateTemplate(dt, title, owner, today, parentID, supersedes, fullWidth)

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(md), 0644); err != nil {
			fmt.Fprintln(stderr, "writing output:", err)
			return exitUnknownErr, err
		}
		fmt.Fprintf(stderr, "template written to %s\n", outputFile)
	} else {
		fmt.Fprint(stdout, md)
	}
	return exitOK, nil
}

// printNewHelp prints usage for `confluence-docs new`.
func printNewHelp(w io.Writer) {
	fmt.Fprintln(w, "new — generate a markdown template for a new Confluence page.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  confluence-docs new <type> --title \"...\" [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Types:")
	fmt.Fprintln(w, "  reference     Static reference doc (PSP, partner, competitor, technology)")
	fmt.Fprintln(w, "  decision      Architecture / product decision record (ADR)")
	fmt.Fprintln(w, "  explanation   Conceptual explanation — the 'why' behind something")
	fmt.Fprintln(w, "  how-to        Step-by-step operational guide")
	fmt.Fprintln(w, "  capture       Quick capture / note (spike, idea, meeting note)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --title TEXT          page title (required)")
	fmt.Fprintln(w, "  --parent-id ID        parent page ID hint (for context; does not create page)")
	fmt.Fprintln(w, "  --supersedes ID       (decision only) page ID of the decision this supersedes")
	fmt.Fprintln(w, "  --full-width          add a note in properties that full-width is intended")
	fmt.Fprintln(w, "  --output FILE         write to file instead of stdout")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "The generated markdown includes a :::properties block, TL;DR section,")
	fmt.Fprintln(w, "and type-specific structural headings. Pipe to 'page create --markdown'.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Workflow:")
	fmt.Fprintln(w, "  # 1. Check for duplicates")
	fmt.Fprintln(w, "  confluence-docs check --title \"My Title\" --type reference")
	fmt.Fprintln(w, "  # 2. Generate template")
	fmt.Fprintln(w, "  confluence-docs new reference --title \"My Title\" --output /tmp/page.md")
	fmt.Fprintln(w, "  # 3. Edit the template, then create")
	fmt.Fprintln(w, "  confluence-docs page create --space-id 131352 --parent-id PARENT --title \"My Title\" --markdown /tmp/page.md")
}

// resolveOwnerEmail reads git config user.email, falling back to $USER@lybel.
func resolveOwnerEmail() string {
	out, err := exec.Command("git", "config", "--global", "user.email").Output()
	if err == nil {
		email := strings.TrimSpace(string(out))
		if email != "" {
			return email
		}
	}
	// Try local git config
	out, err = exec.Command("git", "config", "user.email").Output()
	if err == nil {
		email := strings.TrimSpace(string(out))
		if email != "" {
			return email
		}
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "unknown"
}

// generateTemplate builds the full markdown template for the given doc type.
func generateTemplate(dt docType, title, owner, today, parentID, supersedes string, fullWidth bool) string {
	var sb strings.Builder

	// Properties block
	sb.WriteString(":::properties\n")
	sb.WriteString(fmt.Sprintf("tipo: %s\n", string(dt)))
	sb.WriteString("status: rascunho\n")
	sb.WriteString(fmt.Sprintf("owner: %s\n", owner))
	if parentID != "" {
		sb.WriteString(fmt.Sprintf("parent-id: %s\n", parentID))
	}
	if dt == docTypeDecision && supersedes != "" {
		sb.WriteString(fmt.Sprintf("supersedes: [[id:%s]]\n", supersedes))
	}
	sb.WriteString("relacionados: \"\"\n")
	sb.WriteString(fmt.Sprintf("criado: %s\n", today))
	sb.WriteString(fmt.Sprintf("atualizado: %s\n", today))
	if fullWidth {
		sb.WriteString("layout: full-width\n")
	}
	sb.WriteString(":::\n\n")

	// Shared opening: TL;DR
	sb.WriteString("## TL;DR\n\n")
	sb.WriteString(tldrPlaceholder(dt))
	sb.WriteString("\n\n")

	// Type-specific body
	switch dt {
	case docTypeReference:
		writeReferenceBody(&sb, title)
	case docTypeDecision:
		writeDecisionBody(&sb, title)
	case docTypeExplanation:
		writeExplanationBody(&sb, title)
	case docTypeHowTo:
		writeHowToBody(&sb, title)
	case docTypeCapture:
		writeCaptureBody(&sb, title)
	}

	return sb.String()
}

// tldrPlaceholder returns a one-line TL;DR prompt appropriate for each type.
func tldrPlaceholder(dt docType) string {
	switch dt {
	case docTypeReference:
		return "_Uma frase resumindo o que é, por que importa e qual é o estado atual._"
	case docTypeDecision:
		return "_Decisão: [o que foi decidido]. Motivo: [por quê]. Alternativas consideradas: [lista breve]._"
	case docTypeExplanation:
		return "_Em uma frase, o que este conceito é e por que existe._"
	case docTypeHowTo:
		return "_Quando usar este guia e o que ele entrega ao final._"
	case docTypeCapture:
		return "_Ideia / insight principal em uma frase._"
	default:
		return "_Resumo em uma frase._"
	}
}

func writeReferenceBody(sb *strings.Builder, title string) {
	sb.WriteString("## Contexto\n\n")
	sb.WriteString("_Por que esta página existe? Qual projeto ou decisão a motivou?_\n\n")
	sb.WriteString("## O que é\n\n")
	sb.WriteString(fmt.Sprintf("_Descrição objetiva de `%s`._\n\n", title))
	sb.WriteString("## Como funciona / como usamos\n\n")
	sb.WriteString("_Detalhe técnico, comercial ou operacional relevante._\n\n")
	sb.WriteString("## Status atual\n\n")
	sb.WriteString("_Status, limitações conhecidas, próximos passos._\n\n")
	sb.WriteString("## Links e referências\n\n")
	sb.WriteString("- [documentação oficial](URL)\n")
}

func writeDecisionBody(sb *strings.Builder, title string) {
	sb.WriteString("## Contexto\n\n")
	sb.WriteString("_Qual problema ou oportunidade motivou esta decisão? Qual o escopo?_\n\n")
	sb.WriteString("## Problema\n\n")
	sb.WriteString("_O que dói, quais restrições existem, qual o risco de não decidir?_\n\n")
	sb.WriteString("## Solução adotada\n\n")
	sb.WriteString(fmt.Sprintf("_Descreva a decisão tomada para `%s`._\n\n", title))
	sb.WriteString("## Alternativas consideradas\n\n")
	sb.WriteString("| Alternativa | Prós | Contras | Por que descartada |\n")
	sb.WriteString("|---|---|---|---|\n")
	sb.WriteString("| Alternativa A | | | |\n")
	sb.WriteString("| Alternativa B | | | |\n\n")
	sb.WriteString("## Consequências\n\n")
	sb.WriteString("_O que muda? Quais débitos técnicos ou compromissos esta decisão cria?_\n\n")
	sb.WriteString("## Data de revisão\n\n")
	sb.WriteString("_Esta decisão deve ser revisitada em: YYYY-MM-DD (ou quando evento X ocorrer)._\n")
}

func writeExplanationBody(sb *strings.Builder, title string) {
	sb.WriteString("## Contexto\n\n")
	sb.WriteString("_Para quem é esta explicação? Qual lacuna de conhecimento ela preenche?_\n\n")
	sb.WriteString(fmt.Sprintf("## O que é %s\n\n", title))
	sb.WriteString("_Definição clara, sem jargão interno sem definição._\n\n")
	sb.WriteString("## Por que existe / por que importa\n\n")
	sb.WriteString("_Motivação de negócio ou técnica._\n\n")
	sb.WriteString("## Como se relaciona com o resto\n\n")
	sb.WriteString("_Relacionamentos com outros conceitos, sistemas ou processos do Lybel._\n\n")
	sb.WriteString("## Perguntas frequentes\n\n")
	sb.WriteString("**P:** ...\n**R:** ...\n")
}

func writeHowToBody(sb *strings.Builder, title string) {
	sb.WriteString("## Pré-requisitos\n\n")
	sb.WriteString("_O que a pessoa precisa ter/saber antes de seguir este guia?_\n\n")
	sb.WriteString(fmt.Sprintf("## Como fazer: %s\n\n", title))
	sb.WriteString("1. Passo um\n")
	sb.WriteString("2. Passo dois\n")
	sb.WriteString("3. Passo três\n\n")
	sb.WriteString("## Verificação\n\n")
	sb.WriteString("_Como saber que deu certo?_\n\n")
	sb.WriteString("## Troubleshooting\n\n")
	sb.WriteString("_Problemas comuns e como resolver._\n")
}

func writeCaptureBody(sb *strings.Builder, title string) {
	sb.WriteString("## Contexto\n\n")
	sb.WriteString(fmt.Sprintf("_O que motivou esta captura de `%s`?_\n\n", title))
	sb.WriteString("## Conteúdo\n\n")
	sb.WriteString("_Coloque aqui o conteúdo principal: ideia, insight, nota de reunião, spike._\n\n")
	sb.WriteString("## Próximos passos\n\n")
	sb.WriteString("- [ ] ...\n")
}
