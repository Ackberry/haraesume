package analysis

import (
	"regexp"
	"sort"
	"strings"

	"backend/latex"
)

type SkillCoverage struct {
	Name     string `json:"name"`
	Matched  bool   `json:"matched"`
	Evidence string `json:"evidence,omitempty"`
}

type FitReport struct {
	OverallMatch       float64         `json:"overall_match"`
	RequiredCoverage   float64         `json:"required_coverage"`
	PreferredCoverage  float64         `json:"preferred_coverage"`
	MissingMustHave    []string        `json:"missing_must_have"`
	CoveredStrengths   []string        `json:"covered_strengths"`
	RequiredSkills     []SkillCoverage `json:"required_skills"`
	PreferredSkills    []SkillCoverage `json:"preferred_skills"`
	ATSWarnings        []string        `json:"ats_warnings"`
	WeakBullets        []string        `json:"weak_bullets"`
	TopRecommendations []string        `json:"top_recommendations"`
}

type skillSignal struct {
	Name    string
	Aliases []string
}

var skillSignals = []skillSignal{
	{Name: "Python", Aliases: []string{"python"}},
	{Name: "Go", Aliases: []string{"go", "golang"}},
	{Name: "Java", Aliases: []string{"java"}},
	{Name: "JavaScript", Aliases: []string{"javascript", "js"}},
	{Name: "TypeScript", Aliases: []string{"typescript", "ts"}},
	{Name: "React", Aliases: []string{"react", "react.js", "reactjs"}},
	{Name: "Next.js", Aliases: []string{"next.js", "nextjs"}},
	{Name: "Node.js", Aliases: []string{"node.js", "nodejs", "node"}},
	{Name: "Express.js", Aliases: []string{"express.js", "express"}},
	{Name: "Flask", Aliases: []string{"flask"}},
	{Name: "FastAPI", Aliases: []string{"fastapi"}},
	{Name: "Django", Aliases: []string{"django"}},
	{Name: "HTML/CSS", Aliases: []string{"html", "css", "html/css"}},
	{Name: "C++", Aliases: []string{"c++", "cpp"}},
	{Name: "C", Aliases: []string{"c"}},
	{Name: "Bash", Aliases: []string{"bash", "shell"}},
	{Name: "SQL", Aliases: []string{"sql"}},
	{Name: "PostgreSQL", Aliases: []string{"postgresql", "postgres"}},
	{Name: "MySQL", Aliases: []string{"mysql"}},
	{Name: "MongoDB", Aliases: []string{"mongodb", "mongo"}},
	{Name: "Redis", Aliases: []string{"redis"}},
	{Name: "AWS", Aliases: []string{"aws", "amazon web services"}},
	{Name: "GCP", Aliases: []string{"gcp", "google cloud"}},
	{Name: "Azure", Aliases: []string{"azure"}},
	{Name: "Docker", Aliases: []string{"docker"}},
	{Name: "Kubernetes", Aliases: []string{"kubernetes", "k8s"}},
	{Name: "Terraform", Aliases: []string{"terraform"}},
	{Name: "Git", Aliases: []string{"git"}},
	{Name: "GitHub Actions", Aliases: []string{"github actions"}},
	{Name: "CI/CD", Aliases: []string{"ci/cd", "continuous integration", "continuous delivery"}},
	{Name: "REST APIs", Aliases: []string{"rest api", "rest apis", "restful"}},
	{Name: "GraphQL", Aliases: []string{"graphql"}},
	{Name: "Testing", Aliases: []string{"testing", "unit testing", "integration testing", "test automation"}},
	{Name: "Agile", Aliases: []string{"agile", "scrum"}},
	{Name: "Machine Learning", Aliases: []string{"machine learning", "ml"}},
	{Name: "LLMs", Aliases: []string{"llm", "llms", "large language model"}},
}

var (
	bulletCommandRe = regexp.MustCompile(`\\resumeItem\{([^}]*)\}`)
	emailRe         = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	phoneRe         = regexp.MustCompile(`(\+\d{1,3}[\s-]?)?(\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{4})`)
	plainBulletRe   = regexp.MustCompile(`(?m)^\s*[-*\x{2022}]\s+(.+)$`)
)

func AnalyzeResumeFit(resumeLatex, jobDescription string) FitReport {
	resumeText := latex.ToPlainText(resumeLatex)
	resumeLower := strings.ToLower(resumeText)
	jobLower := strings.ToLower(jobDescription)

	requiredSignals, preferredSignals := classifyJobSkills(jobDescription)
	requiredCoverage := buildCoverage(requiredSignals, resumeLower)
	preferredCoverage := buildCoverage(preferredSignals, resumeLower)

	requiredPct := coveragePercent(requiredCoverage)
	preferredPct := coveragePercent(preferredCoverage)
	overall := round2((requiredPct * 0.7) + (preferredPct * 0.3))

	missing := missingSkillNames(requiredCoverage)
	strengths := matchedSkillNames(append(requiredCoverage, preferredCoverage...))
	atsWarnings := lintATS(resumeLatex, resumeText)
	weakBullets := findWeakBullets(resumeLatex, jobLower)
	recommendations := buildRecommendations(missing, strengths, atsWarnings, weakBullets)

	return FitReport{
		OverallMatch:       overall,
		RequiredCoverage:   requiredPct,
		PreferredCoverage:  preferredPct,
		MissingMustHave:    missing,
		CoveredStrengths:   strengths,
		RequiredSkills:     requiredCoverage,
		PreferredSkills:    preferredCoverage,
		ATSWarnings:        atsWarnings,
		WeakBullets:        weakBullets,
		TopRecommendations: recommendations,
	}
}

func classifyJobSkills(jobDescription string) ([]skillSignal, []skillSignal) {
	lines := latex.SplitInformativeLines(jobDescription)
	required := map[string]skillSignal{}
	preferred := map[string]skillSignal{}
	jobLower := strings.ToLower(jobDescription)

	for _, signal := range skillSignals {
		if !containsAnyTerm(jobLower, signal.Aliases) {
			continue
		}
		isRequired := false
		for _, line := range lines {
			lowerLine := strings.ToLower(line)
			if !containsAnyTerm(lowerLine, signal.Aliases) {
				continue
			}
			if latex.ContainsAny(lowerLine, []string{"required", "must", "minimum", "qualification", "responsibilit", "experience with", "proficiency"}) {
				isRequired = true
				break
			}
		}
		if isRequired {
			required[signal.Name] = signal
		} else {
			preferred[signal.Name] = signal
		}
	}

	requiredList := sortedSignals(required)
	preferredList := sortedSignals(preferred)
	if len(requiredList) == 0 && len(preferredList) > 0 {
		limit := latex.MinInt(5, len(preferredList))
		requiredList = preferredList[:limit]
		preferredList = preferredList[limit:]
	}
	return requiredList, preferredList
}

func sortedSignals(values map[string]skillSignal) []skillSignal {
	out := make([]skillSignal, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func buildCoverage(signals []skillSignal, resumeLower string) []SkillCoverage {
	out := make([]SkillCoverage, 0, len(signals))
	for _, signal := range signals {
		evidence := ""
		matched := false
		for _, alias := range signal.Aliases {
			if containsTerm(resumeLower, alias) {
				matched = true
				evidence = alias
				break
			}
		}
		out = append(out, SkillCoverage{Name: signal.Name, Matched: matched, Evidence: evidence})
	}
	return out
}

func coveragePercent(items []SkillCoverage) float64 {
	if len(items) == 0 {
		return 0
	}
	matched := 0
	for _, item := range items {
		if item.Matched {
			matched++
		}
	}
	return round2(float64(matched) / float64(len(items)) * 100)
}

func missingSkillNames(items []SkillCoverage) []string {
	var out []string
	for _, item := range items {
		if !item.Matched {
			out = append(out, item.Name)
		}
	}
	return out
}

func matchedSkillNames(items []SkillCoverage) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if !item.Matched {
			continue
		}
		if _, ok := seen[item.Name]; ok {
			continue
		}
		seen[item.Name] = struct{}{}
		out = append(out, item.Name)
		if len(out) == 8 {
			break
		}
	}
	return out
}

func lintATS(resumeLatex, resumeText string) []string {
	var warnings []string
	wordCount := len(strings.Fields(resumeText))
	bulletCount := len(extractBullets(resumeLatex))
	lower := strings.ToLower(resumeText)

	if wordCount < 250 {
		warnings = append(warnings, "Resume appears short for internship screening.")
	}
	if bulletCount < 6 {
		warnings = append(warnings, "Add more project or experience bullets with concrete impact.")
	}
	for _, section := range []string{"education", "experience", "technical skills", "projects"} {
		if !strings.Contains(lower, section) {
			warnings = append(warnings, "Missing clear "+section+" section.")
		}
	}
	if !emailRe.MatchString(resumeText) {
		warnings = append(warnings, "Missing email address in contact info.")
	}
	if !phoneRe.MatchString(resumeText) {
		warnings = append(warnings, "Missing phone number in contact info.")
	}
	return warnings
}

func findWeakBullets(resumeLatex, jobLower string) []string {
	bullets := extractBullets(resumeLatex)
	var scored []struct {
		text  string
		score int
	}
	for _, bullet := range bullets {
		plain := strings.TrimSpace(latex.ToPlainText(bullet))
		if plain == "" {
			continue
		}
		lower := strings.ToLower(plain)
		score := 0
		if latex.NumberRegex.MatchString(lower) {
			score += 3
		}
		if overlapsJobSkill(lower, jobLower) {
			score += 2
		}
		if len(strings.Fields(lower)) >= 14 {
			score++
		}
		scored = append(scored, struct {
			text  string
			score int
		}{text: plain, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return len(scored[i].text) < len(scored[j].text)
		}
		return scored[i].score < scored[j].score
	})

	var out []string
	for _, item := range scored {
		if item.score > 2 {
			continue
		}
		out = append(out, item.text)
		if len(out) == 4 {
			break
		}
	}
	return out
}

func extractBullets(resumeLatex string) []string {
	matches := bulletCommandRe.FindAllStringSubmatch(resumeLatex, -1)
	var bullets []string
	for _, match := range matches {
		if len(match) > 1 {
			bullets = append(bullets, match[1])
		}
	}
	if len(bullets) > 0 {
		return bullets
	}
	for _, match := range plainBulletRe.FindAllStringSubmatch(resumeLatex, -1) {
		if len(match) > 1 {
			bullets = append(bullets, match[1])
		}
	}
	return bullets
}

func buildRecommendations(missing, strengths, atsWarnings, weakBullets []string) []string {
	var out []string
	for _, skill := range missing {
		out = append(out, "Add truthful evidence for "+skill+" if you have it.")
		if len(out) == 3 {
			break
		}
	}
	if len(weakBullets) > 0 {
		out = append(out, "Rewrite weak bullets with action, technology, metric, and outcome.")
	}
	if len(strengths) > 0 {
		out = append(out, "Keep strongest matched skills high in Technical Skills.")
	}
	for _, warning := range atsWarnings {
		out = append(out, warning)
		if len(out) == 6 {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, "Resume is broadly aligned; tighten bullets around role-specific impact.")
	}
	return out
}

func overlapsJobSkill(text, jobLower string) bool {
	for _, signal := range skillSignals {
		if containsAnyTerm(text, signal.Aliases) && containsAnyTerm(jobLower, signal.Aliases) {
			return true
		}
	}
	return false
}

func containsAnyTerm(text string, terms []string) bool {
	for _, term := range terms {
		if containsTerm(text, term) {
			return true
		}
	}
	return false
}

func containsTerm(text, term string) bool {
	if text == "" || term == "" {
		return false
	}
	return termRegex(term).MatchString(strings.ToLower(text))
}

func termRegex(term string) *regexp.Regexp {
	term = strings.ToLower(strings.TrimSpace(term))
	return regexp.MustCompile(`(?:^|[^a-z0-9+#])` + regexp.QuoteMeta(term) + `(?:$|[^a-z0-9+#])`)
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
