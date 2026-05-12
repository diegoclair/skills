// cmd_new.go — `confluence-docs new <type>` subcommand.
//
// Generates a markdown template for a new Confluence page of the given type,
// pre-filled with a :::properties block, TL;DR, and the structural headings
// appropriate for the doc type.
//
// Usage:
//
//	confluence-docs new reference   --title "Stripe Brazil Analysis" [--parent-id ID] [--full-width]
//	confluence-docs new decision    --title "..." [--supersedes ID]
//	confluence-docs new explanation --title "..."
//	confluence-docs new how-to      --title "..."
//	confluence-docs new capture     --title "..."
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

// docType is one of the five standard doc types.
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

// runNew implements `confluence-docs new <type>`.
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

// resolveOwnerEmail reads git config user.email, falling back to $USER.
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
	sb.WriteString(fmt.Sprintf("type: %s\n", string(dt)))
	sb.WriteString("status: draft\n")
	sb.WriteString(fmt.Sprintf("owner: %s\n", owner))
	if parentID != "" {
		sb.WriteString(fmt.Sprintf("parent-id: %s\n", parentID))
	}
	if dt == docTypeDecision && supersedes != "" {
		sb.WriteString(fmt.Sprintf("supersedes: [[id:%s]]\n", supersedes))
	}
	sb.WriteString("related: \"\"\n")
	sb.WriteString(fmt.Sprintf("created: %s\n", today))
	sb.WriteString(fmt.Sprintf("updated: %s\n", today))
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
		return "_A sentence summarizing what it is, why it matters, and the current state._"
	case docTypeDecision:
		return "_Decision: [what was decided]. Reason: [why]. Alternatives considered: [brief list]._"
	case docTypeExplanation:
		return "_In one sentence, what this concept is and why it exists._"
	case docTypeHowTo:
		return "_When to use this guide and what it delivers at the end._"
	case docTypeCapture:
		return "_Main idea / insight in one sentence._"
	default:
		return "_One-sentence summary._"
	}
}

func writeReferenceBody(sb *strings.Builder, title string) {
	sb.WriteString("## Context\n\n")
	sb.WriteString("_Why does this page exist? Which project or decision motivated it?_\n\n")
	sb.WriteString("## Identification\n\n")
	sb.WriteString(fmt.Sprintf("_Objective description of `%s`._\n\n", title))
	sb.WriteString("## Attributes\n\n")
	sb.WriteString("_Relevant technical, commercial, or operational detail._\n\n")
	sb.WriteString("## Relevance\n\n")
	sb.WriteString("_Current status, known limitations, next steps._\n\n")
	sb.WriteString("## References\n\n")
	sb.WriteString("- [official documentation](URL)\n")
}

func writeDecisionBody(sb *strings.Builder, title string) {
	sb.WriteString("## Context\n\n")
	sb.WriteString("_What problem or opportunity motivated this decision? What is the scope?_\n\n")
	sb.WriteString("## Problem\n\n")
	sb.WriteString("_What hurts, what constraints exist, what is the risk of not deciding?_\n\n")
	sb.WriteString("## Decision\n\n")
	sb.WriteString(fmt.Sprintf("_Describe the decision taken for `%s`._\n\n", title))
	sb.WriteString("## Alternatives considered\n\n")
	sb.WriteString("| Alternative | Pros | Cons | Why rejected |\n")
	sb.WriteString("|---|---|---|---|\n")
	sb.WriteString("| Alternative A | | | |\n")
	sb.WriteString("| Alternative B | | | |\n\n")
	sb.WriteString("## Consequences\n\n")
	sb.WriteString("_What changes? What technical debt or commitments does this decision create?_\n\n")
	sb.WriteString("## Status and supersession history\n\n")
	sb.WriteString("_This decision should be revisited on: YYYY-MM-DD (or when event X occurs)._\n")
}

func writeExplanationBody(sb *strings.Builder, title string) {
	sb.WriteString("## Context\n\n")
	sb.WriteString("_Who is this explanation for? What knowledge gap does it fill?_\n\n")
	sb.WriteString(fmt.Sprintf("## Analysis: %s\n\n", title))
	sb.WriteString("_Clear definition, no undefined internal jargon._\n\n")
	sb.WriteString("## Implications\n\n")
	sb.WriteString("_Business or technical motivation._\n\n")
	sb.WriteString("## Next steps\n\n")
	sb.WriteString("_Relationships with other concepts, systems, or processes._\n\n")
	sb.WriteString("## FAQ\n\n")
	sb.WriteString("**Q:** ...\n**A:** ...\n")
}

func writeHowToBody(sb *strings.Builder, title string) {
	sb.WriteString("## Prerequisites\n\n")
	sb.WriteString("_What does the reader need to have/know before following this guide?_\n\n")
	sb.WriteString(fmt.Sprintf("## Steps: %s\n\n", title))
	sb.WriteString("1. Step one\n")
	sb.WriteString("2. Step two\n")
	sb.WriteString("3. Step three\n\n")
	sb.WriteString("## Verification\n\n")
	sb.WriteString("_How to know it worked?_\n\n")
	sb.WriteString("## Common issues\n\n")
	sb.WriteString("_Common problems and how to resolve them._\n")
}

func writeCaptureBody(sb *strings.Builder, title string) {
	sb.WriteString("## Idea\n\n")
	sb.WriteString(fmt.Sprintf("_What motivated this capture of `%s`?_\n\n", title))
	sb.WriteString("## Why it might matter\n\n")
	sb.WriteString("_Put the main content here: idea, insight, meeting note, spike._\n\n")
	sb.WriteString("## Suggested next step\n\n")
	sb.WriteString("- [ ] ...\n")
}
