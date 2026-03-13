package llm

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"backend/latex"
)

type SkillCandidate struct {
	Name     string
	Category string
	Keywords []string
	Priority int
}

type SkillSuggestion struct {
	Name     string
	Category string
	Score    int
}

var CuratedSkillCandidates = []SkillCandidate{
	{Name: "PostgreSQL", Category: "Tools", Keywords: []string{"postgresql", "postgres"}, Priority: 6},
	{Name: "MySQL", Category: "Tools", Keywords: []string{"mysql"}, Priority: 4},
	{Name: "Redis", Category: "Tools", Keywords: []string{"redis"}, Priority: 5},
	{Name: "Kubernetes", Category: "Tools", Keywords: []string{"kubernetes", "k8s"}, Priority: 6},
	{Name: "Terraform", Category: "Tools", Keywords: []string{"terraform"}, Priority: 5},
	{Name: "Linux", Category: "Tools", Keywords: []string{"linux"}, Priority: 4},
	{Name: "REST APIs", Category: "Concepts", Keywords: []string{"rest api", "restful"}, Priority: 5},
	{Name: "GraphQL", Category: "Concepts", Keywords: []string{"graphql"}, Priority: 4},
	{Name: "CI/CD", Category: "Concepts", Keywords: []string{"ci/cd", "continuous integration", "continuous delivery"}, Priority: 4},
	{Name: "FastAPI", Category: "Frameworks", Keywords: []string{"fastapi"}, Priority: 6},
	{Name: "Django", Category: "Frameworks", Keywords: []string{"django"}, Priority: 5},
	{Name: "Spring Boot", Category: "Frameworks", Keywords: []string{"spring boot"}, Priority: 5},
	{Name: "PyTorch", Category: "Frameworks", Keywords: []string{"pytorch"}, Priority: 5},
	{Name: "TensorFlow", Category: "Frameworks", Keywords: []string{"tensorflow"}, Priority: 5},
	{Name: "NumPy", Category: "Tools", Keywords: []string{"numpy"}, Priority: 4},
	{Name: "Pandas", Category: "Tools", Keywords: []string{"pandas"}, Priority: 4},
	{Name: "scikit-learn", Category: "Frameworks", Keywords: []string{"scikit-learn", "sklearn"}, Priority: 4},
	{Name: "Apache Kafka", Category: "Tools", Keywords: []string{"kafka", "apache kafka"}, Priority: 5},
	{Name: "RabbitMQ", Category: "Tools", Keywords: []string{"rabbitmq"}, Priority: 4},
	{Name: "Microservices", Category: "Concepts", Keywords: []string{"microservices", "microservice"}, Priority: 5},
	{Name: "Testing", Category: "Concepts", Keywords: []string{"unit testing", "integration testing", "testing"}, Priority: 3},
	{Name: "GitHub Actions", Category: "Tools", Keywords: []string{"github actions"}, Priority: 3},
	{Name: "GitLab CI", Category: "Tools", Keywords: []string{"gitlab ci"}, Priority: 3},
	{Name: "Azure", Category: "Tools", Keywords: []string{"azure"}, Priority: 4},
}

func OptimizeResume(resumeLatex, jobDescription string) (string, string, error) {
	const maxSkillAdds = 5
	targetedSkills := SuggestMissingTechnicalSkills(resumeLatex, jobDescription, maxSkillAdds)

	systemPrompt := `You are an expert resume optimizer. Your primary goal is to improve matching while preserving core resume content.

Hard constraints:
1. Keep all LaTeX syntax valid and compilable.
2. Preserve document structure and formatting commands.
3. Keep the content in Experience, Projects, and Leadership essentially unchanged (same roles, bullets, and claims).
4. Focus edits on the Technical Skills section by adding only a small set of high-value, job-relevant skills.
5. Do not flood the Technical Skills section with every keyword from the job description.
6. Never invent accomplishments, dates, companies, metrics, or responsibilities.

Technical Skills policy:
- Add at most 5 missing skills/tools/frameworks total.
- Prefer skills that are explicit priorities in the job description.
- Keep the original category structure (Languages/Frameworks/Tools/Concepts).
- Preserve existing skills and just append concise additions where appropriate.

Output format:
- First output ONLY the full LaTeX document.
- Then output a separator line exactly: ---CHANGES---
- Then list brief bullet points describing what changed.`

	userPrompt := fmt.Sprintf(`Optimize this resume for the job description using the constraints above.

Job Description:
%s

Recommended missing technical skills to consider (choose only the most important subset, up to 5 total):
%s

Critical section lock:
- Keep Experience, Projects, and Leadership content unchanged apart from tiny wording fixes.

Current Resume (LaTeX):
%s`, jobDescription, formatSkillSuggestions(targetedSkills), resumeLatex)

	content, err := RunLLM(systemPrompt, userPrompt, 4096)
	if err != nil {
		return "", "", err
	}

	optimizedLatex, changesSummary, err := parseOptimizationOutput(content)
	if err != nil {
		return "", "", err
	}

	optimizedLatex = RestoreLockedSections(resumeLatex, optimizedLatex, []string{
		"experience",
		"projects",
		"leadership",
	})

	return optimizedLatex, changesSummary, nil
}

func SuggestMissingTechnicalSkills(resumeLatex, jobDescription string, maxItems int) []SkillSuggestion {
	if maxItems <= 0 {
		return nil
	}
	jobLower := strings.ToLower(jobDescription)
	techSection := strings.ToLower(latex.GetSectionContent(resumeLatex, "technical skills"))
	if techSection == "" {
		techSection = strings.ToLower(resumeLatex)
	}

	candidates := make([]SkillSuggestion, 0, len(CuratedSkillCandidates))
	for _, candidate := range CuratedSkillCandidates {
		if resumeAlreadyHasSkill(techSection, candidate) {
			continue
		}
		matchCount := countSkillMentions(jobLower, candidate.Keywords)
		if matchCount == 0 {
			continue
		}
		score := candidate.Priority + (matchCount * 3) + emphasisBoost(jobLower, candidate.Keywords)
		candidates = append(candidates, SkillSuggestion{
			Name:     candidate.Name,
			Category: candidate.Category,
			Score:    score,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) == 0 {
		return nil
	}

	categoryCap := map[string]int{
		"Languages":  2,
		"Frameworks": 2,
		"Tools":      2,
		"Concepts":   1,
	}
	usedByCategory := map[string]int{}
	out := make([]SkillSuggestion, 0, latex.MinInt(maxItems, len(candidates)))
	for _, suggestion := range candidates {
		if len(out) >= maxItems {
			break
		}
		if usedByCategory[suggestion.Category] >= categoryCap[suggestion.Category] {
			continue
		}
		out = append(out, suggestion)
		usedByCategory[suggestion.Category]++
	}

	return out
}

func RestoreLockedSections(originalLatex, optimizedLatex string, lockedSections []string) string {
	originalSections := latex.ParseSections(originalLatex)
	optimizedSections := latex.ParseSections(optimizedLatex)
	if len(originalSections) == 0 || len(optimizedSections) == 0 {
		return optimizedLatex
	}

	locked := map[string]struct{}{}
	for _, name := range lockedSections {
		locked[latex.NormalizeSectionName(name)] = struct{}{}
	}

	originalByName := map[string]string{}
	for _, section := range originalSections {
		originalByName[latex.NormalizeSectionName(section.Title)] = section.Content
	}

	var builder strings.Builder
	prev := 0
	for _, section := range optimizedSections {
		builder.WriteString(optimizedLatex[prev:section.Start])
		sectionKey := latex.NormalizeSectionName(section.Title)
		if _, shouldLock := locked[sectionKey]; shouldLock {
			if originalContent, ok := originalByName[sectionKey]; ok {
				builder.WriteString(originalContent)
			} else {
				builder.WriteString(section.Content)
			}
		} else {
			builder.WriteString(section.Content)
		}
		prev = section.End
	}
	builder.WriteString(optimizedLatex[prev:])
	return builder.String()
}

func formatSkillSuggestions(suggestions []SkillSuggestion) string {
	if len(suggestions) == 0 {
		return "- No strong missing skills detected from the curated list; keep Technical Skills mostly unchanged."
	}

	var builder strings.Builder
	for _, item := range suggestions {
		builder.WriteString("- ")
		builder.WriteString(item.Name)
		builder.WriteString(" (")
		builder.WriteString(item.Category)
		builder.WriteString(")\n")
	}
	return strings.TrimSpace(builder.String())
}

func countSkillMentions(text string, keywords []string) int {
	count := 0
	for _, keyword := range keywords {
		normalized := strings.TrimSpace(strings.ToLower(keyword))
		if normalized == "" {
			continue
		}
		count += strings.Count(text, normalized)
	}
	return count
}

func emphasisBoost(jobLower string, keywords []string) int {
	lines := latex.SplitInformativeLines(jobLower)
	boost := 0
	for _, line := range lines {
		if !latex.ContainsAny(line, []string{"required", "must", "minimum", "preferred", "qualification", "responsibilit"}) {
			continue
		}
		for _, keyword := range keywords {
			if strings.Contains(line, strings.ToLower(keyword)) {
				boost += 2
				break
			}
		}
	}
	if boost > 6 {
		return 6
	}
	return boost
}

func resumeAlreadyHasSkill(technicalSectionLower string, candidate SkillCandidate) bool {
	for _, keyword := range candidate.Keywords {
		if strings.Contains(technicalSectionLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func parseOptimizationOutput(content string) (string, string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	if raw == "" {
		return "", "", errors.New("model returned empty optimization output")
	}

	parts := strings.SplitN(raw, "---CHANGES---", 2)
	latexSegment := strings.TrimSpace(parts[0])
	changesSummary := ""
	if len(parts) > 1 {
		changesSummary = strings.TrimSpace(parts[1])
	}

	optimizedLatex := latex.ExtractDocument(latexSegment)
	if optimizedLatex == "" {
		optimizedLatex = latex.ExtractDocument(raw)
	}
	if optimizedLatex == "" {
		return "", "", errors.New("model output did not contain a complete LaTeX document")
	}

	if changesSummary == "" {
		remaining := strings.TrimSpace(strings.Replace(raw, optimizedLatex, "", 1))
		remaining = strings.Trim(remaining, "` \n\r\t")
		if remaining != "" && !strings.EqualFold(remaining, "---changes---") {
			changesSummary = remaining
		}
	}

	return optimizedLatex, changesSummary, nil
}
