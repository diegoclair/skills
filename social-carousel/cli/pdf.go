package main

import (
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// combinePDF assembles the provided PNG files into a single multi-page PDF,
// one PNG per page. Page dimensions are derived from the PlatformSpec so that
// the PDF is sized exactly for the target aspect ratio.
//
// Uses pdfcpu's api.ImportImagesFile — pure Go, zero external deps.
// If PDF generation fails it returns an error but does NOT delete the PNGs.
//
// Dimension mapping (logical pixels → PDF points at 72 DPI, 1px = 1pt):
//
//	instagram-4x5  1080×1350 logical → 1080×1350 pt page
//	instagram-1x1  1080×1080 logical → 1080×1080 pt page
//	linkedin-4x5   1080×1350 logical → 1080×1350 pt page
func combinePDF(pngPaths []string, outPath string, spec PlatformSpec) error {
	if len(pngPaths) == 0 {
		return fmt.Errorf("combinePDF: no PNG files provided")
	}

	imp := buildImportConfig(spec)

	// api.ImportImagesFile creates a new PDF at outPath (or appends if it
	// already exists). Pass nil for the conf to use pdfcpu defaults.
	if err := api.ImportImagesFile(pngPaths, outPath, imp, nil); err != nil {
		return fmt.Errorf("combinePDF: pdfcpu import: %w", err)
	}
	return nil
}

// buildImportConfig constructs a pdfcpu Import configuration that sizes
// each page to match the carousel's logical dimensions.
//
// PDF points at 72 DPI: 1 logical px = 1 pt (both are at 72 DPI).
// The image is scaled to fill the entire page (Pos=Full, Scale=1.0,
// ScaleAbs=false) so no white margins appear.
func buildImportConfig(spec PlatformSpec) *pdfcpu.Import {
	// 1 logical CSS px = 1 PDF point (at 72 DPI).
	wPts := float64(spec.Width)
	hPts := float64(spec.Height)

	imp := pdfcpu.DefaultImportConfig()

	// Override the page size with the exact platform dimensions.
	imp.PageDim = &types.Dim{
		Width:  wPts,
		Height: hPts,
	}
	imp.UserDim = true
	imp.PageSize = ""

	// Scale=1.0 + ScaleAbs=false means "scale relative to page, fill it".
	// Pos=Full means the image fills the entire page with no anchor offset.
	imp.Scale = 1.0
	imp.ScaleAbs = false
	imp.Pos = types.Full

	return imp
}
