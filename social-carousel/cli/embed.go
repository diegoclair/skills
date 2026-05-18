package main

import "embed"

// templatesFS holds all template and asset files shipped with the skill.
//
// The physical files live in social-carousel/templates/ (sibling of cli/).
// A symlink cli/templates -> ../templates makes them visible to go:embed,
// which only allows paths that are descendants of the package directory.
//
// Directory layout expected inside templates/:
//
//	templates/
//	  base.css                   ← global reset + CSS variable wiring
//	  carousel.html.tmpl         ← outer HTML shell (head, body, slot for layout)
//	  layouts/
//	    cover.html.tmpl
//	    list.html.tmpl
//	    big-number.html.tmpl
//	    quote.html.tmpl
//	    comparison.html.tmpl
//	    screenshot.html.tmpl
//	    cta.html.tmpl
//	    text.html.tmpl
//	  themes/
//	    dark-tech.yaml
//	    light-editorial.yaml
//	    cream-lifestyle.yaml
//	    neo-brutalist.yaml
//	    minimal-mono.yaml
//	  fonts/
//	    Outfit-Bold.woff2
//	    Outfit-Regular.woff2
//	    DMSans-Regular.woff2
//	    DMSans-Medium.woff2
//	    NotoColorEmoji-Regular.ttf
//
// NOTE: templates/ must be a real directory inside cli/ — NOT a symlink.
// The embed directive does not follow symlinks to directories. All template
// files must live under social-carousel/cli/templates/ directly.
//
// The `all:` prefix ensures dotfiles (.gitkeep) and hidden files are included.
// Without `all:`, go:embed skips files whose names start with `.` or `_`.

//go:embed all:templates
var templatesFS embed.FS
