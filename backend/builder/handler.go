package builder

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"text/template"

	"backend/httputil"
	"backend/latex"
)

type Handler struct {
	tmpl *template.Template
}

func NewHandler() *Handler {
	funcMap := template.FuncMap{
		"esc": EscapeLatex,
		"hasSkills": func(s SkillsData) bool {
			return s.Languages != "" || s.Frameworks != "" || s.Databases != "" || s.Infrastructure != ""
		},
	}
	t := template.Must(template.New("resume").Funcs(funcMap).Parse(resumeTemplate))
	return &Handler{tmpl: t}
}

func (h *Handler) GeneratePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}

	var req BuilderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := validateRequest(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var buf strings.Builder
	if err := h.tmpl.Execute(&buf, req); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "template error: "+err.Error())
		return
	}

	pdfBytes, err := latex.CompileToSinglePagePDF(buf.String())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "pdf compilation failed: "+err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"pdf_base64": base64.StdEncoding.EncodeToString(pdfBytes),
		"filename":   "resume.pdf",
	})
}

const resumeTemplate = `\documentclass[letterpaper,11pt]{article}

\usepackage{latexsym}
\usepackage[empty]{fullpage}
\usepackage{titlesec}
\usepackage{marvosym}
\usepackage[usenames,dvipsnames]{color}
\usepackage{verbatim}
\usepackage{enumitem}
\usepackage[hidelinks]{hyperref}
\usepackage{fancyhdr}
\usepackage[english]{babel}
\usepackage{tabularx}
\usepackage{charter}
\input{glyphtounicode}

\pagestyle{fancy}
\fancyhf{}
\fancyfoot{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\addtolength{\oddsidemargin}{-0.5in}
\addtolength{\evensidemargin}{-0.5in}
\addtolength{\textwidth}{1in}
\addtolength{\topmargin}{-.5in}
\addtolength{\textheight}{1.0in}

\urlstyle{same}
\raggedbottom
\raggedright
\setlength{\tabcolsep}{0in}

\titleformat{\section}{
  \vspace{-4pt}\scshape\raggedright\large
}{}{0em}{}[\color{black}\titlerule \vspace{-5pt}]

\pdfgentounicode=1

\newcommand{\resumeItem}[1]{
  \item\small{
    {#1 \vspace{-2pt}}
  }
}

\newcommand{\resumeSubheading}[4]{
  \vspace{-2pt}\item
    \begin{tabular*}{0.97\textwidth}[t]{l@{\extracolsep{\fill}}r}
      \textbf{#1} & #2 \\
      \textit{\small#3} & \textit{\small #4} \\
    \end{tabular*}\vspace{-7pt}
}

\newcommand{\resumeProjectHeading}[2]{
    \item
    \begin{tabular*}{0.97\textwidth}{l@{\extracolsep{\fill}}r}
      \small#1 & #2 \\
    \end{tabular*}\vspace{-7pt}
}

\newcommand{\resumeSubItem}[1]{\resumeItem{#1}\vspace{-4pt}}

\renewcommand\labelitemii{$\vcenter{\hbox{\tiny$\bullet$}}$}

\newcommand{\resumeSubHeadingListStart}{\begin{itemize}[leftmargin=0.15in, label={}]}
\newcommand{\resumeSubHeadingListEnd}{\end{itemize}}
\newcommand{\resumeItemListStart}{\begin{itemize}}
\newcommand{\resumeItemListEnd}{\end{itemize}\vspace{-5pt}}

\begin{document}

%----------HEADING----------
\begin{center}
    \textbf{\Huge \scshape {{esc .Heading.Name}}} \\ \vspace{1pt}
{{- if .Heading.LinkedIn}}
    \href{ {{- .Heading.LinkedIn -}} }{\underline{LinkedIn}} $|$
{{- end}}
{{- if .Heading.GitHub}}
    \href{ {{- .Heading.GitHub -}} }{\underline{GitHub}} $|$
{{- end}}
    \href{mailto:{{.Heading.Email}}}{\underline{ {{- esc .Heading.Email -}} }}
{{- if .Heading.Phone}} $|$
    {{esc .Heading.Phone}}
{{- end}}
{{- if .Heading.Portfolio}} $|$
    \href{ {{- .Heading.Portfolio -}} }{\underline{Portfolio}}
{{- end}}
\end{center}

{{if .Education -}}
%-----------EDUCATION-----------
\section{Education}
  \resumeSubHeadingListStart
{{range .Education}}    \resumeSubheading
      { {{- esc .School -}} }{ {{- esc .Graduation -}} }
      { {{- esc .Degree -}} }{ {{- esc .Location -}} }
{{- if .Coursework}}
      \vspace{2pt}
      \\ \vspace{4pt}
      \small\textbf{Relevant Coursework:} {{esc .Coursework}}
{{- end}}
{{end}}  \resumeSubHeadingListEnd
{{end}}
{{- if .Experience -}}
%-----------EXPERIENCE-----------
\section{Experience}
  \resumeSubHeadingListStart
{{range .Experience}}
    \resumeSubheading
      { {{- esc .Company -}} }{ {{- esc .Dates -}} }
      { {{- esc .Title -}} }{ {{- esc .Location -}} }
{{- if .Bullets}}
      \resumeItemListStart
{{range .Bullets}}        \resumeItem{ {{- esc . -}} }
{{end}}      \resumeItemListEnd
{{- end}}
{{end}}  \resumeSubHeadingListEnd
{{end}}
{{- if .Projects -}}
%-----------PROJECTS-----------
\section{Projects}
    \resumeSubHeadingListStart
{{range .Projects}}
      \resumeProjectHeading
        { {{- if .Link -}} \textbf{\underline{\href{ {{- .Link -}} }{ {{- esc .Name -}} }}} {{- else -}} \textbf{ {{- esc .Name -}} } {{- end -}} {{- if .TechStack}} $|$ \emph{ {{- esc .TechStack -}} } {{- end -}} }{}
{{- if .Bullets}}
        \resumeItemListStart
{{range .Bullets}}          \resumeItem{ {{- esc . -}} }
{{end}}        \resumeItemListEnd
{{- end}}
{{end}}
    \resumeSubHeadingListEnd
{{end}}
{{- if hasSkills .Skills -}}
%-----------TECHNICAL SKILLS-----------
\section{Technical Skills}
\resumeSubHeadingListStart
  \item \small{
{{- if .Skills.Languages}}
    \textbf{Languages}: {{esc .Skills.Languages}} \\
{{- end}}
{{- if .Skills.Frameworks}}
    \textbf{Frameworks \& Libraries}: {{esc .Skills.Frameworks}} \\
{{- end}}
{{- if .Skills.Databases}}
    \textbf{Databases \& Servers}: {{esc .Skills.Databases}} \\
{{- end}}
{{- if .Skills.Infrastructure}}
    \textbf{Infrastructure \& DevOps}: {{esc .Skills.Infrastructure}}
{{- end}}
  }
\resumeSubHeadingListEnd
{{end}}
{{- if .Leadership -}}
%-----------LEADERSHIP-----------
\section{Leadership}
\resumeSubHeadingListStart
{{range .Leadership}}  \resumeSubheading
    { {{- esc .Organization -}} }{ {{- esc .Dates -}} }
    { {{- esc .Title -}} }{ {{- esc .Location -}} }
{{end}}\resumeSubHeadingListEnd
{{end}}

\end{document}`
