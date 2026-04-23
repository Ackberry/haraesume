package latex

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var usepackageRegex = regexp.MustCompile(`\\usepackage(?:\[[^\]]*\])?\{([^}]*)\}`)

var supportedResumePackages = map[string]struct{}{
	"array":        {},
	"babel":        {},
	"calc":         {},
	"charter":      {},
	"color":        {},
	"enumitem":     {},
	"etoolbox":     {},
	"fancyhdr":     {},
	"fontawesome5": {},
	"fullpage":     {},
	"geometry":     {},
	"graphicx":     {},
	"hyperref":     {},
	"ifthen":       {},
	"latexsym":     {},
	"marvosym":     {},
	"multicol":     {},
	"needspace":    {},
	"parskip":      {},
	"tabularx":     {},
	"titlesec":     {},
	"verbatim":     {},
	"xcolor":       {},
}

type UnsupportedPackagesError struct {
	Packages []string
}

func (e *UnsupportedPackagesError) Error() string {
	if len(e.Packages) == 0 {
		return "unsupported LaTeX packages detected"
	}
	if len(e.Packages) == 1 {
		return fmt.Sprintf(
			"unsupported LaTeX package: %s. Please remove it or switch to a supported resume template",
			e.Packages[0],
		)
	}
	return fmt.Sprintf(
		"unsupported LaTeX packages: %s. Please remove them or switch to a supported resume template",
		strings.Join(e.Packages, ", "),
	)
}

func ExtractUsedPackages(latex string) []string {
	cleaned := commentRegex.ReplaceAllString(latex, "")
	matches := usepackageRegex.FindAllStringSubmatch(cleaned, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	packages := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		for _, rawName := range strings.Split(match[1], ",") {
			name := strings.ToLower(strings.TrimSpace(rawName))
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			packages = append(packages, name)
		}
	}

	sort.Strings(packages)
	return packages
}

func ValidateSupportedPackages(latex string) error {
	packages := ExtractUsedPackages(latex)
	if len(packages) == 0 {
		return nil
	}

	unsupported := make([]string, 0)
	for _, pkg := range packages {
		if _, supported := supportedResumePackages[pkg]; supported {
			continue
		}
		unsupported = append(unsupported, pkg)
	}
	if len(unsupported) == 0 {
		return nil
	}
	return &UnsupportedPackagesError{Packages: unsupported}
}
