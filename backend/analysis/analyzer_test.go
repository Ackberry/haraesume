package analysis

import (
	"os"
	"strings"
	"testing"
)

func TestAnalyzeResumeFitSoftwareInternshipSample(t *testing.T) {
	resume, err := os.ReadFile("../state/base_resume.tex")
	if err != nil {
		t.Fatalf("read sample resume: %v", err)
	}

	jobDescription := `Software Engineering Intern

Responsibilities:
- Build and ship full-stack features using React, TypeScript, Node.js, and REST APIs.
- Work with SQL databases and cloud infrastructure to support reliable services.
- Write unit tests, participate in code reviews, and collaborate in an Agile team.

Minimum qualifications:
- Experience with Python or Go.
- Experience with JavaScript or TypeScript.
- Experience building web applications with React.
- Familiarity with Git.

Preferred qualifications:
- Docker, Kubernetes, PostgreSQL, AWS, and CI/CD experience.`

	report := AnalyzeResumeFit(string(resume), jobDescription)
	if report.OverallMatch <= 0 {
		t.Fatalf("OverallMatch = %v, want positive score", report.OverallMatch)
	}
	allCoverage := append(report.RequiredSkills, report.PreferredSkills...)
	if !coverageMatched(allCoverage, "React") {
		t.Fatalf("Coverage = %v, want React matched", allCoverage)
	}
	if !containsString(report.MissingMustHave, "Go") {
		t.Fatalf("MissingMustHave = %v, want Go", report.MissingMustHave)
	}

	t.Logf("overall=%0.0f required=%0.0f preferred=%0.0f", report.OverallMatch, report.RequiredCoverage, report.PreferredCoverage)
	t.Logf("missing=%s", strings.Join(report.MissingMustHave, ", "))
	t.Logf("strengths=%s", strings.Join(report.CoveredStrengths, ", "))
	t.Logf("recommendations=%s", strings.Join(report.TopRecommendations, " | "))
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func coverageMatched(items []SkillCoverage, target string) bool {
	for _, item := range items {
		if item.Name == target && item.Matched {
			return true
		}
	}
	return false
}
