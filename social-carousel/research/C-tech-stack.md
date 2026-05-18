# Tech Stack Research: HTML+CSS → Instagram Carousel PNGs

**Date:** 2026-05-17
**Context:** Pure Go skill (no Node) that renders HTML+CSS templates into individual PNGs (2160×2700 or 2160×2160 @2x) plus an optional combined PDF. Reference: LeaderPro's `marketing/scripts/export-carousel.mjs` (Puppeteer + pdf-lib, 96 lines).

---

## 1. Candidates evaluated

### 1a. chromedp
- **Repo:** https://github.com/chromedp/chromedp
- **Stars:** ~13k | **Last commit:** Apr/2026 (v0.15.1) | **Open issues:** 166
- Pure-Go CDP (Chrome DevTools Protocol). Zero Node deps. Spawns an external Chrome/Chromium process via `ExecAllocator`. Relevant APIs: `CaptureScreenshot`, `FullScreenshot`, `ScreenshotNodes`.
- Chrome is not embedded in the Go binary — it has to be installed on the host (or shipped via the `chromedp/headless-shell` Docker image, ~500 MB compressed). The resulting Go binary is small (~15-25 MB), but the external dep is Chrome itself (~260 MB on disk on Linux).
- Native headless since Chrome 112 (`--headless=new`). No XVFB required.
- Cold start: ~1-3 s (Chrome process boot); subsequent calls reuse the instance.

### 1b. go-rod / rod
- **Repo:** https://github.com/go-rod/rod
- **Stars:** ~6.9k | **Last commit:** Jul/2024 (v0.116.2) | **Open issues:** 179
- Also a CDP client in Go, but with a higher-level abstraction and automatic browser download via `BrowserManager`. Decode-on-demand (more efficient than chromedp on event-heavy workloads). Thread-safe by design.
- Supports `MustScreenshot`, `MustScreenshotFullPage`, and per-element screenshots.
- **Caveat:** the last release was Jul/2024 — ~10 months without a release. There are recent commits, but the project appears less active than chromedp.
- No XVFB required. Cold start similar to chromedp.

### 1c. playwright-go
- **Repo:** https://github.com/playwright-community/playwright-go
- **Stars:** ~3.3k | **Last commit:** Feb/2026 (v0.5700.1) | **Open issues:** 69
- Community wrapper (not official Microsoft). Architecture: Go communicates over stdio with a bundled Node.js + Playwright runtime (~50 MB extra). In other words, **it is not pure Go** — it embeds Node internally.
- Supports Chromium, Firefox, and WebKit.
- Higher cold start because of the Node + browser bootstrap.
- The hidden Node dependency makes deployment more complex without any clear win over chromedp.

### 1d. Puppeteer (Node)
- What LeaderPro uses today. It works, but it requires both the Node runtime and Chromium installed.
- For 2160×2700 screenshots: `page.setViewport({width:1080, height:1350, deviceScaleFactor:2})` → 2x PNG.
- Warm instance: ~200-400 ms per screenshot. Cold (fresh process): 2-5 s.
- RAM: ~200-400 MB per Chrome instance.

### 1e. wkhtmltoimage
- **Site:** https://wkhtmltopdf.org/
- Qt WebKit engine. **Archived project** — last release in 2020. Modern CSS (full flexbox, CSS grid, gradients with `backdrop-filter`, CSS variables) has partial or buggy support.
- No XVFB required — truly headless. Lightweight (~100 MB binary + libs).
- **Custom fonts:** read via fontconfig; need to be installed on the system or in `/usr/local/share/fonts/`. Known bugs with dynamic `@font-face`.
- **Verdict:** rejected. The engine is outdated; PNG output still works but CSS fidelity is inferior.

### 1f. WeasyPrint (Python)
- Version 53+ **removed PNG output**. It only emits PDF now. Getting PNG would require converting the PDF with another tool.
- Adds a Python dep to the pipeline. It uses its own CSS layout engine, not a real browser — fidelity for modern CSS gradients and Google Fonts is limited.
- **Verdict:** out of scope. PNG was dropped and it is Python-based.

### 1g. resvg + Go bindings
- **Main repo (Rust):** https://github.com/linebender/resvg — active, v0.47.0 (Feb/2026)
- **Go binding:** https://github.com/xo/resvg (wraps the resvg C-API via cgo)
- Renders SVG → PNG with excellent quality (Skia/tiny-skia). No browser.
- **Critical limitation:** it only renders static SVG. It does not execute JavaScript and does not consume HTML+CSS directly. We would have to convert HTML+CSS templates to SVG first — a non-trivial flow that loses fidelity.
- No XVFB. Go binary with cgo: ~30-50 MB.
- **Verdict:** not suitable for rich HTML+CSS templates. Suitable only for pure SVG.

### 1h. Headless Chrome via direct CLI
- `google-chrome --headless=new --screenshot=out.png --window-size=2160,2700 file.html`
- `--force-device-scale-factor=2` flag for 2x DPI.
- Zero Go library overhead — just `exec.Command`.
- Fidelity identical to chromedp (same engine). Google Fonts resolve if the template uses a local `@font-face`.
- **Downside:** limited flow control (no wait-for-element, no JS injection to signal "page ready"). For static templates with no async JS, it works perfectly.
- Cold start: ~2-4 s per invocation (no process reuse). Worse than chromedp for multi-slide carousels.

### 1i. Bun + Puppeteer
- Puppeteer runs on the Bun runtime (compatibility improved through 2024-2025).
- Performance is marginally better than Node (Bun startup is ~30-50 ms faster), but the bottleneck is Chrome, not the JS runtime.
- In practice: behaves the same as Puppeteer + Node for this use case.
- **Verdict:** does not justify switching stacks.

---

## 2. Comparison table

| Option | External dep | Cold start | Render fidelity | Cross-platform | Maturity | Go binary | XVFB? |
|---|---|---|---|---|---|---|---|
| **chromedp** | Chrome/Chromium ~260 MB | 1-3 s (reuses instance) | ★★★★★ (real Chrome) | macOS/Linux/Win | ★★★★★ 13k stars, active Apr/2026 | ~15-25 MB | No |
| **go-rod** | Chrome/Chromium ~260 MB | 1-3 s | ★★★★★ (real Chrome) | macOS/Linux/Win | ★★★☆☆ 6.9k, last release Jul/2024 | ~20-30 MB | No |
| **playwright-go** | Node + Playwright ~50 MB + Chrome | 2-5 s | ★★★★★ (real Chrome) | macOS/Linux/Win | ★★★☆☆ 3.3k, community | ~15 MB + Node | No |
| **Puppeteer (Node)** | Node + Chrome | 2-5 s cold | ★★★★★ (real Chrome) | macOS/Linux/Win | ★★★★★ industry reference | N/A (not Go) | No |
| **wkhtmltoimage** | Qt WebKit ~100 MB | ~0.5-1 s | ★★☆☆☆ (2020 WebKit) | macOS/Linux/Win | ★☆☆☆☆ archived 2020 | N/A (C binary) | No |
| **WeasyPrint** | Python + Cairo | ~1-2 s | ★★★☆☆ (custom CSS) | macOS/Linux/Win | ★★★☆☆ active 2026 | N/A (Python) | No |
| **resvg + Go** | Rust cgo libs ~10 MB | <100 ms | ★★★★☆ (SVG only) | macOS/Linux/Win | ★★★★☆ v0.47 Feb/2026 | ~30-50 MB | No |
| **Chrome CLI direct** | Chrome ~260 MB | 2-4 s per slide | ★★★★★ (real Chrome) | macOS/Linux/Win | ★★★★★ (native Chrome) | ~5 MB (exec wrapper) | No |
| **Bun + Puppeteer** | Bun + Chrome | 1.5-4 s | ★★★★★ (real Chrome) | macOS/Linux/Win | ★★★★☆ Bun active | N/A (not Go) | No |

---

## 3. Final recommendations

### 3a. Winning stack — render HTML+CSS → PNG

**Recommendation: `chromedp`**

Rationale:
- Pure Go, zero Node deps — aligns with the constraint of the parent skill `confluence-docs`.
- More stars and more active than go-rod (13k vs 6.9k, release Apr/2026 vs Jul/2024).
- Reuses the Chrome instance via `context` — one cold start per skill run, not per slide. A 10-slide carousel = 1 cold start + 10 fast captures (~200-400 ms each).
- Scale/viewport API: `chromedp.EmulateViewport(width, height, deviceScaleFactor)` to render at 2x.
- For 2160×2700: `EmulateViewport(1080, 1350, 2.0)` → native 2x capture.
- No XVFB needed — runs on Linux CI/CD without a display.
- Fonts and emojis: see sections 3c and 3d below.

Minimal flow example:
```go
ctx, cancel := chromedp.NewContext(allocCtx)
defer cancel()

var buf []byte
chromedp.Run(ctx,
    chromedp.EmulateViewport(1080, 1350, chromedp.EmulateScale(2.0)),
    chromedp.Navigate("file:///path/to/slide1.html"),
    chromedp.WaitVisible(`body`),
    chromedp.CaptureScreenshot(&buf),
)
os.WriteFile("slide1.png", buf, 0644)
```

**Plan B:** direct Chrome CLI via `exec.Command` if chromedp shows issues on some OS. For static templates (no async JS), `--headless=new --screenshot --window-size=2160,2700 --force-device-scale-factor=1` works without an extra library. Bottleneck: no process reuse — each slide opens and closes Chrome (~2-4 s per slide).

### 3b. Winning stack — combine PNGs into PDF

**Recommendation: `pdfcpu`**

- **Repo:** https://github.com/pdfcpu/pdfcpu
- Stars: 8.6k | Last commit: May/2026 (v0.12.1) — extremely active.
- Pure Go, zero external deps for combining images into a PDF.
- Direct API: `api.ImportImagesFile([]string{"s1.png","s2.png",...}, "out.pdf", imp, nil)` — produces a multi-page PDF with each PNG on its own page.
- Alternative via `io.Reader`: `api.ImportImages(rs, w, []io.Reader{...}, imp, nil)` — zero intermediate disk I/O.
- Configurable page format: `pdfcpu.Import{PageDim: &types.Dim{W:612, H:792}}` or a custom size for LinkedIn (A4/Letter or 4:5 aspect).
- Why not `gofpdf`: archived project (last commit 2021). Why not `unipdf`: commercial (paid license for production).

### 3c. Offline Google Fonts

**Recommended strategy: static embed in the templates**

Do not download at runtime. The HTML template should include fonts as local assets:

```
skill/
  templates/
    carousel/
      assets/
        fonts/
          Outfit-Bold.woff2
          DMSans-Regular.woff2
          NotoColorEmoji-Regular.ttf
      base.css        ← @font-face pointing to ./assets/fonts/
      slide.html.tmpl
```

`base.css`:
```css
@font-face {
  font-family: 'Outfit';
  src: url('./assets/fonts/Outfit-Bold.woff2') format('woff2');
  font-weight: 700;
}
@font-face {
  font-family: 'DM Sans';
  src: url('./assets/fonts/DMSans-Regular.woff2') format('woff2');
  font-weight: 400;
}
```

chromedp renders via the `file://` URI — Chrome loads local `@font-face` files without issue. No network requests.

Fonts to ship in the skill bundle:
- Outfit Bold + Regular (heading): ~120 KB each (WOFF2)
- DM Sans Regular + Medium (body): ~100 KB each (WOFF2)
- Noto Color Emoji: see section 3d

Tool for downloading the WOFF2 files: [google-webfonts-helper](https://gwfh.mranftl.com/fonts) — generates ready-to-use CSS plus the WOFF2 files.

**Lazy download alternative (if bundle size is a concern):** on first run, the skill detects missing files and downloads from `fonts.googleapis.com`, saving them to `~/.cache/lybel-carousel/fonts/`. More complex; not recommended for v1.

### 3d. Color emojis

**Recommended strategy: local `@font-face` with NotoColorEmoji (COLRv1)**

Headless Chrome (v112+) supports COLRv1 — the correct format for color emojis on Chromium. The `fontsource/noto-color-emoji` npm package uses SVG tables (which work on Safari, not Chrome). Use the TTF version directly:

1. Download `NotoColorEmoji-Regular.ttf` from https://github.com/googlefonts/noto-emoji/tree/main/fonts (~10 MB)
2. Add it to the skill's asset bundle:

```css
@font-face {
  font-family: 'Noto Color Emoji';
  src: url('./assets/fonts/NotoColorEmoji-Regular.ttf') format('truetype');
}

body {
  font-family: 'Outfit', 'DM Sans', 'Noto Color Emoji', sans-serif;
}
```

3. Headless Chrome renders color emojis natively with this setup.

**Alternative without the 10 MB bundle:** install `fonts-noto-color-emoji` via apt on the CI/Linux environment (`apt-get install -y fonts-noto-color-emoji`) and let Chrome pick it up via fontconfig. Works on Linux CI but is not cross-platform portable without the local file.

**Final recommendation for the skill:** ship NotoColorEmoji-Regular.ttf in the bundle. 10 MB is reasonable for an image-generation CLI, and it guarantees color emojis on any OS without user-side configuration.

### 3e. Plan B by platform

| OS | Failure scenario | Plan B |
|---|---|---|
| macOS | Chrome not installed | `brew install --cask google-chrome` or `chromedp/headless-shell` via Docker |
| Linux CI | Missing libs (libglib, libnss) | `apt-get install -y chromium-browser` + point `ExecAllocator` to the chromium path |
| Windows | Different Chrome path | `chromedp.FindChrome()` auto-detects; fallback: `--exec-path C:\...\chrome.exe` |
| Any OS | Chrome unavailable | Fallback to Chrome CLI (`exec.Command`) with `--headless=new`; no context reuse |

---

## 4. Decision Tree — `render(template, data) → []png`

```
render(template, data) → []png
│
├── 1. SETUP (once per skill run)
│   ├── Find Chrome: chromedp.FindChrome() or LYBEL_CAROUSEL_CHROME_PATH env
│   ├── If not found → clear error: "install Chrome or set LYBEL_CAROUSEL_CHROME_PATH"
│   ├── allocCtx ← chromedp.NewExecAllocator(ctx,
│   │       chromedp.NoFirstRun,
│   │       chromedp.NoDefaultBrowserCheck,
│   │       chromedp.Headless,
│   │       chromedp.DisableGPU,
│   │       chromedp.Flag("disable-web-security", true),  // allows file:// @font-face
│   │       chromedp.Flag("allow-file-access-from-files", true),
│   │   )
│   └── browserCtx ← chromedp.NewContext(allocCtx)
│
├── 2. PRE-PROCESSING
│   ├── Render Go template: slides []html ← template.Execute(data)
│   ├── Write slides to tmpDir: /tmp/lybel-carousel-<uuid>/slide-{01..N}.html
│   │       (each HTML includes <link rel=stylesheet href=./assets/base.css>)
│   └── Validate: len(slides) > 0 and len(slides) <= 20
│
├── 3. RENDER LOOP (reuses browserCtx)
│   └── For each slide i, html:
│       ├── slideCtx, cancel ← chromedp.NewContext(browserCtx)
│       ├── chromedp.Run(slideCtx,
│       │       EmulateViewport(1080, 1350, scale=2.0),   // → 2160×2700 px
│       │       Navigate("file://" + tmpDir + "/slide-i.html"),
│       │       WaitVisible("body"),
│       │       WaitFunc("() => document.fonts.ready"),   // waits for @font-face to load
│       │       CaptureScreenshot(&buf),
│       │   )
│       ├── On error → retry once; if it still fails → return error with slide index
│       ├── Validate len(buf) > 0
│       ├── Save to outDir/slide-{01..N}.png
│       └── cancel()
│
├── 4. OPTIONAL PDF (--pdf or --linkedin flag)
│   ├── pngs ← list of generated PNG paths
│   ├── imp ← &pdfcpu.Import{PageDim: pageDimForAspect(aspectRatio)}
│   ├── api.ImportImagesFile(pngs, outDir+"/carousel.pdf", imp, nil)
│   └── On error → log warning (do not fail; PNGs are already generated)
│
├── 5. CLEANUP
│   ├── Remove tmpDir
│   └── Close browserCtx
│
└── 6. RETURN
    └── []string{outDir/slide-01.png, ..., outDir/slide-N.png}
        + optional: outDir/carousel.pdf
```

---

## 5. Performance estimates

| Slides | Estimated time (chromedp, warm instance) | RAM |
|---|---|---|
| 5 | ~3-5 s (1 cold start + 5×400 ms) | ~300 MB (Chrome) |
| 10 | ~5-8 s | ~350 MB |
| 20 | ~10-15 s | ~400 MB |

For an interactive CLI (not serverless), these numbers are acceptable. If the skill later runs in CI/CD with many parallel carousels, consider a pool of Chrome instances via `chromedp.NewContext(browserCtx)` with goroutine workers.

---

## 6. Final skill dependencies

```
go.mod:
  github.com/chromedp/chromedp  v0.15.x   ← render HTML → PNG
  github.com/pdfcpu/pdfcpu      v0.12.x   ← combine PNGs → PDF

assets (bundled with the skill, not downloaded at runtime):
  assets/fonts/Outfit-Bold.woff2           ~120 KB
  assets/fonts/Outfit-Regular.woff2        ~110 KB
  assets/fonts/DMSans-Regular.woff2        ~100 KB
  assets/fonts/DMSans-Medium.woff2         ~105 KB
  assets/fonts/NotoColorEmoji-Regular.ttf  ~10 MB

System dep (installed by the user, not by the skill):
  Google Chrome or Chromium >= v112
```

Total added Go deps: 2 packages, zero external Node/Python binaries.

---

## Sources consulted

- [chromedp GitHub](https://github.com/chromedp/chromedp)
- [chromedp pkg.go.dev](https://pkg.go.dev/github.com/chromedp/chromedp)
- [go-rod GitHub](https://github.com/go-rod/rod)
- [playwright-go GitHub](https://github.com/playwright-community/playwright-go)
- [pdfcpu GitHub](https://github.com/pdfcpu/pdfcpu)
- [pdfcpu pkg.go.dev API](https://pkg.go.dev/github.com/pdfcpu/pdfcpu/pkg/api)
- [resvg Go bindings (xo/resvg)](https://github.com/xo/resvg)
- [resvg Rust library](https://github.com/linebender/resvg)
- [WeasyPrint PNG removal — issue](https://github.com/Kozea/WeasyPrint/issues/1200)
- [Noto Color Emoji self-hosted](https://github.com/infolektuell/noto-color-emoji)
- [googlefonts/noto-emoji](https://github.com/googlefonts/noto-emoji)
- [Chrome Headless docs](https://developer.chrome.com/docs/chromium/headless)
- [ZenRows — chromedp tutorial 2026](https://www.zenrows.com/blog/chromedp)
- [ZenRows — playwright-go 2026](https://www.zenrows.com/blog/playwright-golang)
- [Puppeteer vs Playwright performance 2025](https://www.skyvern.com/blog/puppeteer-vs-playwright-complete-performance-comparison-2025/)
- [wkhtmltopdf.org](https://wkhtmltopdf.org/)
- [Google Fonts self-hosting knowledge](https://fonts.google.com/knowledge/using_type/self_hosting_web_fonts)
- [google-webfonts-helper](https://gwfh.mranftl.com/fonts)
- [UniDoc — Golang PDF Library Guide](https://unidoc.io/post/golang-pdf-library-guide/)
- [Best Headless Browser Tools 2026 — NodeMaven](https://nodemaven.com/blog/headless-browser/)
