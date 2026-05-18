// Package main — types defines the input schema for a carousel.
//
// The YAML schema is the contract between the LLM agent and the renderer.
// It is deliberately minimal: the agent passes only the content + a theme
// reference; everything else (fonts, sizes, colors, spacing) is resolved
// from the theme. This keeps the agent payload token-efficient and lets
// users iterate on visuals without changing copy.
package main

// Carousel is the top-level document a user (or an LLM agent) authors.
type Carousel struct {
	// Platform target. Determines export aspect, output format, safe zones.
	// One of: "instagram-4x5", "instagram-1x1", "linkedin-4x5".
	// Default: "instagram-4x5".
	Platform string `yaml:"platform,omitempty" json:"platform,omitempty"`

	// Theme name (preset shipped with the skill) OR path to a custom
	// theme YAML on disk. Presets: "dark-tech", "light-editorial",
	// "cream-lifestyle", "neo-brutalist", "minimal-mono".
	// Default: "dark-tech".
	Theme string `yaml:"theme,omitempty" json:"theme,omitempty"`

	// Handle shown at the bottom of every slide (e.g. "@lybel.com.br").
	// Empty = no handle (the brand-anchor footer is omitted).
	Handle string `yaml:"handle,omitempty" json:"handle,omitempty"`

	// Logo path (PNG/SVG). Resolved relative to the YAML file. Optional;
	// if set it is shown next to the handle. Max rendered size 60×60 px.
	Logo string `yaml:"logo,omitempty" json:"logo,omitempty"`

	// ShowSlideNumber controls the "N/T" indicator at the bottom-right.
	// Default: true.
	ShowSlideNumber *bool `yaml:"show_slide_number,omitempty" json:"show_slide_number,omitempty"`

	// Slides is the ordered list. 1-20 entries. The first slide is the
	// cover; the last is typically a CTA. Slide 3 is the "value bomb"
	// (algorithmic inflection point — research A, ST-05).
	Slides []Slide `yaml:"slides" json:"slides"`

	// CaptionSeed is the post caption suggestion. Not rendered into any
	// slide — printed by `render` so the agent can paste it into Instagram.
	CaptionSeed string `yaml:"caption_seed,omitempty" json:"caption_seed,omitempty"`

	// Hashtags suggested for the caption. Same as CaptionSeed: not rendered.
	Hashtags []string `yaml:"hashtags,omitempty" json:"hashtags,omitempty"`
}

// Slide is one frame of the carousel. The Layout field switches which
// template + which fields are read. Unknown fields for a layout are
// silently ignored (forward-compat: a v0.2 template can add new fields
// without breaking older YAMLs).
type Slide struct {
	// Layout selects the template. Required.
	// One of: "cover", "list", "big-number", "quote", "comparison",
	// "screenshot", "cta", "text".
	Layout string `yaml:"layout" json:"layout"`

	// HookFormula is an optional tag identifying which of the 13 hook
	// formulas this slide uses (H-01..H-13). Not rendered; used by the
	// linter for tracking and by future analytics.
	HookFormula string `yaml:"hook_formula,omitempty" json:"hook_formula,omitempty"`

	// --- cover ---
	Label string `yaml:"label,omitempty" json:"label,omitempty"`   // small uppercase tag at top, e.g. "SOLO PROVIDER"
	Hook  string `yaml:"hook,omitempty" json:"hook,omitempty"`     // gigantic headline, ≤12 words
	Sub   string `yaml:"sub,omitempty" json:"sub,omitempty"`       // microcopy line below the hook

	// --- list ---
	Title string   `yaml:"title,omitempty" json:"title,omitempty"`
	Items []string `yaml:"items,omitempty" json:"items,omitempty"` // ≤5 items per slide

	// --- big-number ---
	Number  string `yaml:"number,omitempty" json:"number,omitempty"`   // e.g. "87%"
	Caption string `yaml:"caption,omitempty" json:"caption,omitempty"` // short punchline ≤6 words; the number does the headline work
	// Subhead renders ABOVE the number as a small uppercase eyebrow (e.g.
	// "TEMPO ATÉ ZERAR" before "25 MIN"). Optional. Use when the number alone
	// is ambiguous to a cold reader. ≤6 words. Independent of `headline` (CTA).
	Subhead string `yaml:"subhead,omitempty" json:"subhead,omitempty"`
	// Context renders BELOW the caption as a single explanatory line (e.g.
	// "of independent service providers lose clients due to lack of follow-up").
	// Optional. Use when the caption can't carry the story on its own. ≤20 words.
	Context string `yaml:"context,omitempty" json:"context,omitempty"`

	// --- quote ---
	Quote       string `yaml:"quote,omitempty" json:"quote,omitempty"`             // body, ≤25 words
	Attribution string `yaml:"attribution,omitempty" json:"attribution,omitempty"` // "Name, Context"

	// --- comparison ---
	BeforeLabel string   `yaml:"before_label,omitempty" json:"before_label,omitempty"`
	BeforeItems []string `yaml:"before_items,omitempty" json:"before_items,omitempty"`
	AfterLabel  string   `yaml:"after_label,omitempty" json:"after_label,omitempty"`
	AfterItems  []string `yaml:"after_items,omitempty" json:"after_items,omitempty"`
	// Orientation controls how the two sides are laid out:
	//   "" or "auto"  — vertical (top/bottom) when both sides have ≤2 items,
	//                   horizontal (left/right) otherwise. Optimises canvas use.
	//   "horizontal"  — force two columns side-by-side.
	//   "vertical"    — force two rows stacked (antes on top, depois below).
	Orientation string `yaml:"orientation,omitempty" json:"orientation,omitempty"`

	// --- screenshot ---
	Image      string `yaml:"image,omitempty" json:"image,omitempty"` // relative path to screenshot
	DeviceKind string `yaml:"device,omitempty" json:"device,omitempty"`   // "iphone" | "browser" | "android" | "" (none)

	// --- cta (last slide) ---
	Headline string `yaml:"headline,omitempty" json:"headline,omitempty"` // question or command, ≤12 words
	CTAText  string `yaml:"cta_text,omitempty" json:"cta_text,omitempty"` // "Comment LYBEL", "Save and DM me"
	SwipeBack bool  `yaml:"swipe_back,omitempty" json:"swipe_back,omitempty"` // shows "← back to start"

	// --- text (free-form fallback; use sparingly) ---
	Body string `yaml:"body,omitempty" json:"body,omitempty"`

	// Tone overrides the carousel's theme bg/fg/accent for one slide.
	// Empty = inherit theme as-is. Values:
	//   "authority" — theme's primary dark variant (dark bg, light fg)
	//   "clarity"   — theme's light variant (light bg, dark fg)
	//   "spotlight" — accent-color background, white fg
	// Use sparingly: rotation A/B/A/B/B/B/B/C/A/A is the research default for 10 slides.
	Tone string `yaml:"tone,omitempty" json:"tone,omitempty"`

	// HookStyle controls cover-hook rendering. Empty = solid (default).
	//   "gradient" — render the hook as a linear gradient between accent and accent_alt.
	//                Requires the theme to have accent_alt set; falls back to solid otherwise.
	HookStyle string `yaml:"hook_style,omitempty" json:"hook_style,omitempty"`

	// Note is a per-slide free-form field for agent-only context (notes
	// to self, A/B variant tags). Never rendered.
	Note string `yaml:"note,omitempty" json:"note,omitempty"`
}

// Theme is the visual contract: tokens that base.css consumes via
// CSS variables. Themes live as YAML files under templates/themes/
// (presets) or are user-authored and referenced by path.
type Theme struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`

	// Colors. Hex strings, e.g. "#0D1117".
	BgPrimary   string `yaml:"bg_primary"`              // main slide background
	BgSecondary string `yaml:"bg_secondary,omitempty"`  // optional alt bg for variation
	FgPrimary   string `yaml:"fg_primary"`              // body text
	FgSecondary string `yaml:"fg_secondary,omitempty"`  // muted text (caption, handle)
	Accent      string `yaml:"accent"`                  // primary brand color
	AccentAlt   string `yaml:"accent_alt,omitempty"`    // secondary accent

	// Typography. Family names must match what is registered in
	// base.css via @font-face. Presets ship with Outfit + DM Sans;
	// custom themes can declare new font files in their own dir.
	FontHeading string `yaml:"font_heading"`            // e.g. "Outfit"
	FontBody    string `yaml:"font_body"`               // e.g. "DM Sans"
	FontQuote   string `yaml:"font_quote,omitempty"`    // optional, used by quote layout

	// Decorative options.
	BackgroundEffect string `yaml:"background_effect,omitempty"` // "" | "dots" | "grid" | "halo"
	BorderStyle      string `yaml:"border_style,omitempty"`      // "" | "frame" | "accent-line" | "neo-brutalist"
}

// PlatformSpec is internal — resolved from the Carousel.Platform string.
// Not part of the user-facing YAML.
type PlatformSpec struct {
	Name           string // "instagram-4x5"
	Width          int    // logical width in CSS px (1080)
	Height         int    // logical height (1350 for 4x5)
	DeviceScale    float64 // 2.0 for retina export → 2160×2700
	OutputFormat   string // "png" | "pdf"
	AspectShortcut string // "4:5" | "1:1" | "9:16"
}

// platformSpecs is the canonical table of supported export targets.
// Add a new entry to extend the skill to a new platform.
var platformSpecs = map[string]PlatformSpec{
	"instagram-4x5": {
		Name: "instagram-4x5", Width: 1080, Height: 1350, DeviceScale: 2.0,
		OutputFormat: "png", AspectShortcut: "4:5",
	},
	"instagram-1x1": {
		Name: "instagram-1x1", Width: 1080, Height: 1080, DeviceScale: 2.0,
		OutputFormat: "png", AspectShortcut: "1:1",
	},
	"linkedin-4x5": {
		Name: "linkedin-4x5", Width: 1080, Height: 1350, DeviceScale: 2.0,
		OutputFormat: "pdf", AspectShortcut: "4:5",
	},
}

// resolvePlatform returns the spec for c.Platform, with default fallback.
func resolvePlatform(c *Carousel) PlatformSpec {
	if c.Platform == "" {
		return platformSpecs["instagram-4x5"]
	}
	if spec, ok := platformSpecs[c.Platform]; ok {
		return spec
	}
	// Unknown platform — fall back, the linter will flag it.
	return platformSpecs["instagram-4x5"]
}
