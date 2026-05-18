package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// loadCarousel reads and parses a carousel YAML file.
// The returned Carousel has been normalized (default platform, default
// theme) but NOT linted — the linter is a separate step.
func loadCarousel(path string) (*Carousel, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", abs, err)
	}
	var c Carousel
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if c.Platform == "" {
		c.Platform = "instagram-4x5"
	}
	if c.Theme == "" {
		c.Theme = "dark-tech"
	}
	if c.ShowSlideNumber == nil {
		t := true
		c.ShowSlideNumber = &t
	}
	return &c, nil
}
