package llm

import (
	"strings"
	"testing"
)

func TestPrioritizeTechnicalSkillsMovesAddedSkillsToFront(t *testing.T) {
	original := `\documentclass{article}
\begin{document}
\section{Technical Skills}
\resumeSubHeadingListStart
  \item \small{
    \textbf{Tools}: Docker, Git, Jira \\
    \textbf{Concepts}: Backend, Frontend
  }
\resumeSubHeadingListEnd
\end{document}`

	optimized := `\documentclass{article}
\begin{document}
\section{Technical Skills}
\resumeSubHeadingListStart
  \item \small{
    \textbf{Tools}: Docker, Git, Jira, Kubernetes, PostgreSQL \\
    \textbf{Concepts}: Backend, Frontend, REST APIs
  }
\resumeSubHeadingListEnd
\end{document}`

	jobDescription := "Must have Kubernetes and PostgreSQL experience. Required REST API design."

	got := PrioritizeTechnicalSkills(original, optimized, jobDescription)

	assertContains(t, got, `\textbf{Tools}: Kubernetes, PostgreSQL, Docker, Git, Jira \\`)
	assertContains(t, got, `\textbf{Concepts}: REST APIs, Backend, Frontend`)
}

func TestPrioritizeTechnicalSkillsReordersExistingSkillsByJobPriority(t *testing.T) {
	original := `\documentclass{article}
\begin{document}
\section{Technical Skills}
\resumeSubHeadingListStart
  \item \small{
    \textbf{Languages}: JavaScript, Python, Go, TypeScript \\
    \textbf{Frameworks}: React, Flask, Next.js
  }
\resumeSubHeadingListEnd
\end{document}`

	optimized := original
	jobDescription := "Required: Python, Python, Go. Preferred Next.js experience."

	got := PrioritizeTechnicalSkills(original, optimized, jobDescription)

	assertContains(t, got, `\textbf{Languages}: Python, Go, JavaScript, TypeScript \\`)
	assertContains(t, got, `\textbf{Frameworks}: Next.js, React, Flask`)
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q\n\ngot:\n%s", want, got)
	}
}
