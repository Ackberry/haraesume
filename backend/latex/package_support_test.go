package latex

import (
	"errors"
	"slices"
	"testing"
)

func TestExtractUsedPackages(t *testing.T) {
	source := `
\documentclass{article}
% \usepackage{ignored}
\usepackage[hidelinks]{hyperref}
\usepackage{xcolor, graphicx}
\usepackage{fontawesome5}
`

	got := ExtractUsedPackages(source)
	want := []string{"fontawesome5", "graphicx", "hyperref", "xcolor"}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractUsedPackages() = %v, want %v", got, want)
	}
}

func TestValidateSupportedPackagesAllowsTierOneSet(t *testing.T) {
	source := `
\documentclass{article}
\usepackage{geometry}
\usepackage{xcolor,graphicx}
\usepackage{multicol}
\usepackage{fontawesome5}
`

	if err := ValidateSupportedPackages(source); err != nil {
		t.Fatalf("ValidateSupportedPackages() unexpected error: %v", err)
	}
}

func TestValidateSupportedPackagesRejectsUnsupported(t *testing.T) {
	source := `
\documentclass{article}
\usepackage{geometry}
\usepackage{fontspec}
\usepackage{fontawesome5}
`

	err := ValidateSupportedPackages(source)
	if err == nil {
		t.Fatal("ValidateSupportedPackages() error = nil, want unsupported package error")
	}

	var unsupportedErr *UnsupportedPackagesError
	if !errors.As(err, &unsupportedErr) {
		t.Fatalf("ValidateSupportedPackages() error = %T, want *UnsupportedPackagesError", err)
	}
	if !slices.Equal(unsupportedErr.Packages, []string{"fontspec"}) {
		t.Fatalf("Unsupported packages = %v, want [fontspec]", unsupportedErr.Packages)
	}
}
