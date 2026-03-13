package llm

import (
	"errors"
	"fmt"

	"backend/latex"
)

func GenerateCoverLetter(resumeLatex, jobDescription string) (string, error) {
	resumePlain := latex.ToPlainText(resumeLatex)
	resumeHighlights := pickResumeHighlights(resumePlain, 8)
	jobHighlights := pickJobHighlights(jobDescription, 8)

	if len(resumeHighlights) == 0 {
		resumeHighlights = []string{"No clear resume bullet highlights were extracted; use only verified input details."}
	}
	if len(jobHighlights) == 0 {
		jobHighlights = []string{"Prioritize the role's strongest requirement and align to relevant candidate evidence."}
	}

	systemPrompt := `You are an expert cover letter writer for technical roles.

Write a polished, formal cover letter as a complete, compilable LaTeX document.

Hard requirements:
1. Output only LaTeX from \documentclass to \end{document}.
2. Use an article-based letter layout with formal structure:
   - Candidate header block (name and available contact details)
   - Date line (\today)
   - Hiring manager / company block (use details from job description when present)
   - Salutation ("Dear Hiring Manager," if no specific name)
   - 3 to 4 body paragraphs
   - Professional closing ("Sincerely,") with candidate name
3. Keep body content to 220-380 words.
4. Include at least two concrete resume-backed examples (tools, impact, metrics, scope).
5. Address at least two specific job priorities.
6. Never invent facts.
7. Keep LaTeX simple and robust for pdflatex:
   - \documentclass[11pt]{article}
   - \usepackage[margin=1in]{geometry}
   - \usepackage[hidelinks]{hyperref}
   - \setlength{\parindent}{0pt}
   - \setlength{\parskip}{0.8em}
8. Escape LaTeX special characters when needed.
9. Do not include markdown fences, explanations, or placeholders like [Company Name] unless the data is truly unavailable.`

	userPrompt := fmt.Sprintf(`Write a tailored cover letter using only the factual context below.

Job description:
%s

Top job priorities:
%s

Resume highlights:
%s

Resume plaintext context (for fact-checking):
%s`,
		latex.TruncateForPrompt(jobDescription, 5000),
		latex.FormatBullets(jobHighlights),
		latex.FormatBullets(resumeHighlights),
		latex.TruncateForPrompt(resumePlain, 6000),
	)

	draftTemp := 0.65
	draft, err := RunLLMWithTemperature(systemPrompt, userPrompt, 2200, &draftTemp)
	if err != nil {
		return "", err
	}
	draftLatex, err := parseCoverLetterOutput(draft)
	if err != nil {
		return "", err
	}

	refineSystemPrompt := `You are a senior editor improving a LaTeX cover letter draft.

Preserve factual accuracy and keep the output as a full compilable LaTeX document.

Improve:
1. Formality and business-letter polish
2. Specific alignment to the role
3. Concision and readability
4. LaTeX correctness (no broken commands, no markdown)

Return only final LaTeX from \documentclass to \end{document}.`

	refinePrompt := fmt.Sprintf(`Job description:
%s

Top job priorities:
%s

Resume highlights:
%s

Draft to improve:
%s`,
		latex.TruncateForPrompt(jobDescription, 5000),
		latex.FormatBullets(jobHighlights),
		latex.FormatBullets(resumeHighlights),
		latex.TruncateForPrompt(draftLatex, 7000),
	)

	refineTemp := 0.35
	refined, err := RunLLMWithTemperature(refineSystemPrompt, refinePrompt, 2200, &refineTemp)
	if err != nil {
		return draftLatex, nil
	}

	finalLatex, err := parseCoverLetterOutput(refined)
	if err != nil {
		return draftLatex, nil
	}
	return finalLatex, nil
}

func parseCoverLetterOutput(content string) (string, error) {
	doc := latex.ExtractDocument(content)
	if doc == "" {
		return "", errors.New("model output did not contain a complete LaTeX cover letter document")
	}
	return doc, nil
}

func pickResumeHighlights(resumeText string, maxItems int) []string {
	lines := latex.SplitInformativeLines(resumeText)
	scored := latex.ScoreLines(lines, []string{
		"built", "developed", "designed", "led", "improved", "reduced", "increased",
		"launched", "shipped", "optimized", "automated", "implemented",
	}, []string{
		"python", "go", "java", "javascript", "typescript", "react", "node", "sql",
		"aws", "gcp", "azure", "docker", "kubernetes", "llm", "langchain", "langgraph",
	})
	return latex.TopScoredLines(scored, maxItems)
}

func pickJobHighlights(jobDescription string, maxItems int) []string {
	lines := latex.SplitInformativeLines(jobDescription)
	scored := latex.ScoreLines(lines, []string{
		"required", "must", "minimum", "preferred", "responsible", "responsibilities",
		"qualifications", "experience", "skills", "ability", "expect",
	}, []string{
		"python", "go", "java", "javascript", "typescript", "react", "node", "sql",
		"aws", "gcp", "azure", "docker", "kubernetes", "llm", "langchain", "langgraph",
	})
	return latex.TopScoredLines(scored, maxItems)
}
