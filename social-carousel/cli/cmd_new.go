package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// runNew scaffolds a starter YAML for a given carousel "kind".
//
// The kinds map to the structural archetypes documented in research A —
// each one produces a 8–10 slide skeleton that already satisfies the
// linter's structural rules (cover first, value-bomb at slide 3, CTA
// last). The agent fills in the actual copy.
//
// Kinds:
//   listicle     — H-13 hook + 6 numbered list items + recap + CTA
//   case-study   — H-03 confessional + before/after + result + CTA
//   framework    — H-01 result-promise + 6 framework steps + recap + CTA
//   comparison   — H-02 contrarian + comparison slides + CTA
//   story        — H-11 transformation + narrative + payoff + CTA
//   data-drop    — H-07 authority + big-number slides + list + CTA
func runNew(args []string, stdout, stderr io.Writer) (int, error) {
	var kind, outPath string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--out":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--out requires a path")
				return exitInputErr, errInvalidUsage
			}
			outPath = args[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "new — scaffold a starter YAML for a carousel.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  social-carousel new <kind> [--out FILE]")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Kinds: listicle | case-study | framework | comparison | story | data-drop")
			return exitOK, nil
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintln(stderr, "unknown flag:", a)
				return exitInputErr, errInvalidUsage
			}
			if kind != "" {
				fmt.Fprintln(stderr, "only one kind at a time")
				return exitInputErr, errInvalidUsage
			}
			kind = a
		}
	}
	if kind == "" {
		fmt.Fprintln(stderr, "missing kind")
		return exitInputErr, errInvalidUsage
	}
	tmpl, ok := scaffolds[kind]
	if !ok {
		fmt.Fprintf(stderr, "unknown kind %q — try: listicle | case-study | framework | comparison | story | data-drop\n", kind)
		return exitInputErr, errInvalidUsage
	}
	if outPath == "" {
		fmt.Fprint(stdout, tmpl)
		return exitOK, nil
	}
	if err := os.WriteFile(outPath, []byte(tmpl), 0o644); err != nil {
		fmt.Fprintln(stderr, "write:", err)
		return exitUnknownErr, err
	}
	fmt.Fprintln(stdout, "Wrote:", outPath)
	return exitOK, nil
}

// scaffolds maps kind -> starter YAML. Each scaffold is "lint-clean by
// construction": it follows the 32 rules (≤12 words in cover hook,
// value-bomb at slide 3, CTA last, etc.) so the agent only has to fill
// in real copy without re-thinking structure.
var scaffolds = map[string]string{
	"listicle": `# Listicle carousel (H-13 hook: "N things nobody tells you — #4 is the worst").
# 10 slides; item #4 delivers the "value bomb" promised in the hook.
platform: instagram-4x5
theme: dark-tech
handle: "@yourhandle"
slides:
  - layout: cover
    hook_formula: H-13
    label: "CATEGORY"
    hook: "7 mistakes that cost you clients (#4 is fatal)"
    sub: "swipe →"
  - layout: text
    body: "Before mistake #1: I thought this was obvious. It isn't."
  - layout: big-number  # <-- slide 3 = value bomb
    number: "87%"
    caption: "make mistake #4 without realizing"
  - layout: list
    title: "The first 3 mistakes"
    items:
      - "Mistake 1 — short description"
      - "Mistake 2 — short description"
      - "Mistake 3 — short description"
  - layout: text
    body: "Mistake #4: what makes you lose clients without noticing."
  - layout: list
    title: "The last 3"
    items:
      - "Mistake 5 — short description"
      - "Mistake 6 — short description"
      - "Mistake 7 — short description"
  - layout: quote
    quote: "Checklist: what to stop doing this week."
    attribution: "you, after saving this"
  - layout: cta
    headline: "Want to fix all these mistakes?"
    cta_text: "Comment CHECKLIST and I'll send the template"
    swipe_back: true
caption_seed: |
  7 mistakes that cost you clients (#4 is fatal).
  Save it to review before next week.
hashtags: ["#soloprovider"]
`,
	"framework": `# Framework carousel — H-01 result-promise.
platform: instagram-4x5
theme: dark-tech
handle: "@yourhandle"
slides:
  - layout: cover
    hook_formula: H-01
    label: "FRAMEWORK"
    hook: "The system that doubles your retention in 30 days"
    sub: "save it to apply this week →"
  - layout: text
    body: "Why retention matters more than acquisition right now."
  - layout: big-number  # value bomb
    number: "5×"
    caption: "cheaper to retain than to acquire"
  - layout: list
    title: "Steps 1–3"
    items:
      - "Step 1 — clear action"
      - "Step 2 — clear action"
      - "Step 3 — clear action"
  - layout: list
    title: "Steps 4–6"
    items:
      - "Step 4 — clear action"
      - "Step 5 — clear action"
      - "Step 6 — clear action"
  - layout: screenshot
    title: "How it looks in practice"
    image: "./assets/screenshot.png"
    device: "iphone"
    caption: "Example of the framework applied"
  - layout: quote
    quote: "Apply it before adding 1 more client."
    attribution: "golden rule"
  - layout: cta
    headline: "Want to apply it with me?"
    cta_text: "Comment FRAMEWORK and I'll send the template"
    swipe_back: true
caption_seed: |
  Retention framework: 6 steps I apply with my clients.
hashtags: []
`,
	"case-study": `# Case study — H-03 confessional mistake.
platform: instagram-4x5
theme: light-editorial
handle: "@yourhandle"
slides:
  - layout: cover
    hook_formula: H-03
    label: "REAL CASE"
    hook: "I lost 8 clients in 60 days. Here's what I learned."
    sub: "swipe →"
  - layout: text
    body: "How I was operating before."
  - layout: big-number
    number: "−42%"
    caption: "revenue drop in Q1"
  - layout: comparison
    before_label: "BEFORE"
    before_items:
      - "No follow-up"
      - "No fixed schedule"
    after_label: "AFTER"
    after_items:
      - "Follow-up on D+7"
      - "Weekly schedule"
  - layout: list
    title: "Concrete changes"
    items:
      - "Change 1"
      - "Change 2"
      - "Change 3"
  - layout: big-number
    number: "+18"
    caption: "clients recovered in 30 days"
  - layout: quote
    quote: "A returning client is worth 3× a new one."
    attribution: "an expensive lesson"
  - layout: cta
    headline: "Want to see the step-by-step?"
    cta_text: "Save this post and DM me"
    swipe_back: true
caption_seed: |
  I lost 8 clients in 60 days. What changed and what came back.
hashtags: []
`,
	"comparison": `# Comparison — H-02 counter-intuitive.
platform: instagram-4x5
theme: minimal-mono
handle: "@yourhandle"
slides:
  - layout: cover
    hook_formula: H-02
    label: "CONTRARIAN"
    hook: "Posting every day hurts (do less, better)"
    sub: "swipe →"
  - layout: text
    body: "Why high posting frequency sabotages the algorithm."
  - layout: big-number
    number: "3×"
    caption: "more reach with fewer posts"
  - layout: comparison
    before_label: "WRONG"
    before_items:
      - "1 generic post/day"
      - "No topic repetition"
    after_label: "RIGHT"
    after_items:
      - "3 focused posts/week"
      - "Fixed pillars"
  - layout: comparison
    before_label: "WRONG"
    before_items:
      - "Vanity metrics"
    after_label: "RIGHT"
    after_items:
      - "Saves + DMs"
  - layout: list
    title: "How to apply this week"
    items:
      - "Item 1"
      - "Item 2"
      - "Item 3"
  - layout: quote
    quote: "Frequency without focus is just noise."
    attribution: "test it and tell me"
  - layout: cta
    headline: "Down to test it for 7 days?"
    cta_text: "Comment TEST and I'll send the plan"
    swipe_back: true
caption_seed: |
  Posting every day hurts. How to rebalance your frequency.
hashtags: []
`,
	"story": `# Story — H-11 personal transformation.
platform: instagram-4x5
theme: cream-lifestyle
handle: "@yourhandle"
slides:
  - layout: cover
    hook_formula: H-11
    label: "MY STORY"
    hook: "I used to think talent was everything. I was wrong."
    sub: "swipe →"
  - layout: text
    body: "How it was at the start."
  - layout: big-number
    number: "0 → 100"
    caption: "clients in 18 months"
  - layout: list
    title: "What changed"
    items:
      - "Learned to delegate"
      - "Documented processes"
      - "Focused on retention"
  - layout: quote
    quote: "Talent opens the door. Systems keep it open."
    attribution: "lesson from the turning point"
  - layout: screenshot
    title: "Where I am today"
    image: "./assets/result.png"
    device: "browser"
    caption: "Dashboard screenshot, May/2026"
  - layout: cta
    headline: "Ready to start your turnaround?"
    cta_text: "Save this post"
    swipe_back: true
caption_seed: |
  How I went from 0 to 100 clients in 18 months.
hashtags: []
`,
	"data-drop": `# Data drop — H-07 borrowed authority.
platform: instagram-4x5
theme: duotone-deep
handle: "@yourhandle"
slides:
  - layout: cover
    hook_formula: H-07
    label: "RESEARCH"
    hook: "I analyzed 500 profiles. Here's what 9/10 do alike."
    sub: "swipe →"
  - layout: text
    body: "The research: sample, method, scope."
  - layout: big-number
    number: "87%"
    caption: "share the same hook pattern"
  - layout: big-number
    number: "12s"
    caption: "average time on slide 1"
  - layout: list
    title: "The 5 dominant patterns"
    items:
      - "Pattern 1"
      - "Pattern 2"
      - "Pattern 3"
      - "Pattern 4"
      - "Pattern 5"
  - layout: list
    title: "What sets the top 10% apart"
    items:
      - "Differentiator 1"
      - "Differentiator 2"
      - "Differentiator 3"
  - layout: quote
    quote: "Data without action is just noise."
    attribution: "the question is: what do you change?"
  - layout: cta
    headline: "Want the full study?"
    cta_text: "Comment STUDY and I'll send it"
    swipe_back: true
caption_seed: |
  Analyzed 500 profiles. 5 dominant patterns of the top 10%.
hashtags: []
`,
}
