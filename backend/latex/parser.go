package latex

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	fenceRegex   = regexp.MustCompile("(?is)```(?:latex)?\\s*(.*?)```")
	commentRegex = regexp.MustCompile(`(?m)%.*$`)
	commandRegex = regexp.MustCompile(`\\[A-Za-z]+[*]?`)
	SectionRE    = regexp.MustCompile(`(?m)^\s*\\section\{([^}]+)\}`)
	SpaceRegex   = regexp.MustCompile(`[ \t]+`)
	NumberRegex  = regexp.MustCompile(`\b\d+(?:[.,]\d+)?(?:%|x|k|m|ms)?\b`)
)

type Section struct {
	Title   string
	Start   int
	End     int
	Content string
}

func ExtractDocument(content string) string {
	cleaned := strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	if cleaned == "" {
		return ""
	}

	fencedMatches := fenceRegex.FindAllStringSubmatch(cleaned, -1)
	for _, match := range fencedMatches {
		if len(match) < 2 {
			continue
		}
		if doc := TrimToDocument(match[1]); doc != "" {
			return doc
		}
	}

	return TrimToDocument(cleaned)
}

func TrimToDocument(content string) string {
	start := strings.Index(content, "\\documentclass")
	end := strings.LastIndex(content, "\\end{document}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	end += len("\\end{document}")
	return strings.TrimSpace(content[start:end])
}

func ParseSections(latex string) []Section {
	matches := SectionRE.FindAllStringSubmatchIndex(latex, -1)
	if len(matches) == 0 {
		return nil
	}

	sections := make([]Section, 0, len(matches))
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}
		start := match[0]
		end := len(latex)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		title := strings.TrimSpace(latex[match[2]:match[3]])
		sections = append(sections, Section{
			Title:   title,
			Start:   start,
			End:     end,
			Content: latex[start:end],
		})
	}
	return sections
}

func GetSectionContent(latex string, sectionName string) string {
	target := NormalizeSectionName(sectionName)
	for _, section := range ParseSections(latex) {
		if NormalizeSectionName(section.Title) == target {
			return section.Content
		}
	}
	return ""
}

func NormalizeSectionName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func ToPlainText(latex string) string {
	cleaned := commentRegex.ReplaceAllString(latex, "")
	cleaned = strings.NewReplacer(
		`\\`, "\n",
		`~`, " ",
		`_`, " ",
		`^`, " ",
		`$`, " ",
		`{`, " ",
		`}`, " ",
		`&`, " and ",
	).Replace(cleaned)
	cleaned = commandRegex.ReplaceAllString(cleaned, " ")

	lines := strings.Split(cleaned, "\n")
	output := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(SpaceRegex.ReplaceAllString(line, " "))
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "documentclass") {
			continue
		}
		output = append(output, line)
	}
	return strings.Join(output, "\n")
}

type ScoredLine struct {
	Text  string
	Score int
}

func SplitInformativeLines(text string) []string {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.TrimSpace(strings.TrimLeft(raw, "-*• \t"))
		line = strings.TrimSpace(SpaceRegex.ReplaceAllString(line, " "))
		if len(line) < 20 {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func ScoreLines(lines []string, priorityTerms []string, skillTerms []string) []ScoredLine {
	seen := map[string]struct{}{}
	scored := make([]ScoredLine, 0, len(lines))
	for _, line := range lines {
		normalized := strings.ToLower(line)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}

		score := 1
		if NumberRegex.MatchString(normalized) {
			score += 3
		}
		if ContainsAny(normalized, priorityTerms) {
			score += 2
		}
		if ContainsAny(normalized, skillTerms) {
			score++
		}
		if len(strings.Fields(normalized)) >= 18 {
			score++
		}
		scored = append(scored, ScoredLine{Text: line, Score: score})
	}
	return scored
}

func TopScoredLines(lines []ScoredLine, maxItems int) []string {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].Score == lines[j].Score {
			return len(lines[i].Text) > len(lines[j].Text)
		}
		return lines[i].Score > lines[j].Score
	})

	if maxItems <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) > maxItems {
		lines = lines[:maxItems]
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, line.Text)
	}
	return out
}

func ContainsAny(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func FormatBullets(items []string) string {
	if len(items) == 0 {
		return "- (none)"
	}
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString("- ")
		builder.WriteString(strings.TrimSpace(item))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func TruncateForPrompt(text string, maxRunes int) string {
	trimmed := strings.TrimSpace(text)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "\n...[truncated]"
}

func LooksLikeCompanyName(value string) bool {
	lower := strings.ToLower(value)
	if lower == "" {
		return false
	}

	for _, badTerm := range []string{
		"responsibilities",
		"requirements",
		"qualification",
		"job description",
		"position",
		"benefits",
		"salary",
		"location",
		"remote",
		"full-time",
		"full time",
		"internship",
	} {
		if strings.Contains(lower, badTerm) {
			return false
		}
	}

	companyHints := []string{
		"inc",
		"llc",
		"corp",
		"corporation",
		"technologies",
		"technology",
		"systems",
		"labs",
		"group",
		"company",
		"solutions",
	}
	if ContainsAny(lower, companyHints) {
		return true
	}

	words := strings.Fields(value)
	if len(words) == 0 || len(words) > 8 {
		return false
	}

	capitalized := 0
	for _, word := range words {
		trimmed := strings.Trim(word, ".,;:()[]{}")
		if trimmed == "" {
			continue
		}
		r, _ := utf8.DecodeRuneInString(trimmed)
		if r >= 'A' && r <= 'Z' {
			capitalized++
		}
	}
	return capitalized >= len(words)-1
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
