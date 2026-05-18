# Font Assets — social-carousel

This directory must contain the WOFF2 font files listed below.
The files are **NOT committed to the repository** (listed in .gitignore) because
Google Fonts licenses require redistribution only in web contexts.

Run `make fonts` from the `social-carousel/` directory to download everything.

---

## Expected files

```
templates/fonts/
├── Outfit-700.woff2
├── Outfit-800.woff2
├── DMSans-400.woff2
├── DMSans-500.woff2
├── DMSans-700.woff2
├── PlayfairDisplay-700.woff2
├── PlayfairDisplay-700italic.woff2
├── SpaceGrotesk-600.woff2
├── SpaceGrotesk-700.woff2
├── Inter-400.woff2
├── Inter-500.woff2
└── NotoColorEmoji-Regular.ttf
```

---

## Download commands (manual)

### Via google-webfonts-helper (recommended — gives WOFF2 with no subsetting)

Base URL: `https://gwfh.mranftl.com/api/fonts/<font-id>?download=zip&subsets=latin&variants=<weight>`

```bash
# Outfit 700 and 800
curl -fsSL "https://gwfh.mranftl.com/api/fonts/outfit?download=zip&subsets=latin&variants=700,800" -o /tmp/outfit.zip
unzip -j /tmp/outfit.zip "*.woff2" -d templates/fonts/
# Rename to canonical names:
mv templates/fonts/outfit-v11-latin-700.woff2    templates/fonts/Outfit-700.woff2   2>/dev/null || true
mv templates/fonts/outfit-v11-latin-800.woff2    templates/fonts/Outfit-800.woff2   2>/dev/null || true

# DM Sans 400, 500, 700
curl -fsSL "https://gwfh.mranftl.com/api/fonts/dm-sans?download=zip&subsets=latin&variants=400,500,700" -o /tmp/dmsans.zip
unzip -j /tmp/dmsans.zip "*.woff2" -d templates/fonts/
mv templates/fonts/dm-sans-v15-latin-regular.woff2 templates/fonts/DMSans-400.woff2 2>/dev/null || true
mv templates/fonts/dm-sans-v15-latin-500.woff2     templates/fonts/DMSans-500.woff2 2>/dev/null || true
mv templates/fonts/dm-sans-v15-latin-700.woff2     templates/fonts/DMSans-700.woff2 2>/dev/null || true

# Playfair Display 700 and 700italic
curl -fsSL "https://gwfh.mranftl.com/api/fonts/playfair-display?download=zip&subsets=latin&variants=700,700italic" -o /tmp/playfair.zip
unzip -j /tmp/playfair.zip "*.woff2" -d templates/fonts/
mv templates/fonts/playfair-display-v37-latin-700.woff2        templates/fonts/PlayfairDisplay-700.woff2       2>/dev/null || true
mv templates/fonts/playfair-display-v37-latin-700italic.woff2  templates/fonts/PlayfairDisplay-700italic.woff2 2>/dev/null || true

# Space Grotesk 600 and 700
curl -fsSL "https://gwfh.mranftl.com/api/fonts/space-grotesk?download=zip&subsets=latin&variants=600,700" -o /tmp/spacegrotesk.zip
unzip -j /tmp/spacegrotesk.zip "*.woff2" -d templates/fonts/
mv templates/fonts/space-grotesk-v16-latin-600.woff2 templates/fonts/SpaceGrotesk-600.woff2 2>/dev/null || true
mv templates/fonts/space-grotesk-v16-latin-700.woff2 templates/fonts/SpaceGrotesk-700.woff2 2>/dev/null || true

# Inter 400 and 500
curl -fsSL "https://gwfh.mranftl.com/api/fonts/inter?download=zip&subsets=latin&variants=400,500" -o /tmp/inter.zip
unzip -j /tmp/inter.zip "*.woff2" -d templates/fonts/
mv templates/fonts/inter-v13-latin-regular.woff2 templates/fonts/Inter-400.woff2 2>/dev/null || true
mv templates/fonts/inter-v13-latin-500.woff2     templates/fonts/Inter-500.woff2 2>/dev/null || true
```

### Noto Color Emoji (TTF — not available via gwfh)

```bash
# From the official googlefonts/noto-emoji GitHub release:
curl -fsSL \
  "https://github.com/googlefonts/noto-emoji/raw/main/fonts/NotoColorEmoji.ttf" \
  -o templates/fonts/NotoColorEmoji-Regular.ttf
```

---

## Note on file naming

The `Makefile` `fonts` target handles download AND renaming automatically.
The `@font-face` declarations in `base.css` expect the canonical names above
exactly — if you download manually, rename accordingly.

Font version numbers in filenames (e.g. `outfit-v11-latin-700.woff2`) vary as
Google Fonts updates fonts. The Makefile uses a glob rename (`mv *outfit*700* ...`)
to handle version changes without script maintenance.
