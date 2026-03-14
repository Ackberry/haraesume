package latex

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type compiler struct {
	Command string
	Args    []string
	Runs    int
}

func CompileToPDF(latexSource string) ([]byte, error) {
	pdf, _, err := compileToPDFInternal(latexSource)
	return pdf, err
}

func compileToPDFInternal(latexSource string) ([]byte, string, error) {
	tempDir, err := os.MkdirTemp("", "resume_*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	latexSource = strings.TrimSpace(strings.TrimPrefix(latexSource, "\ufeff"))
	if normalized := ExtractDocument(latexSource); normalized != "" {
		latexSource = normalized
	}
	if latexSource == "" {
		return nil, "", errors.New("no LaTeX source to compile")
	}

	texPath := filepath.Join(tempDir, "resume.tex")
	pdfPath := filepath.Join(tempDir, "resume.pdf")

	if err := os.WriteFile(texPath, []byte(latexSource), 0o600); err != nil {
		return nil, "", fmt.Errorf("failed to write LaTeX source: %w", err)
	}

	compilerLog, err := runCompiler(tempDir, texPath)
	if err != nil {
		return nil, compilerLog, err
	}

	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, compilerLog, errors.New("failed to read PDF output")
	}

	return pdfBytes, compilerLog, nil
}

func runCompiler(tempDir, texPath string) (string, error) {
	texFilename := filepath.Base(texPath)
	compilers := []compiler{
		{
			Command: "latexmk",
			Args: []string{
				"-pdf",
				"-interaction=nonstopmode",
				"-halt-on-error",
				"-file-line-error",
				"-output-directory=" + tempDir,
				texFilename,
			},
			Runs: 1,
		},
		{
			Command: "tectonic",
			Args: []string{
				"--keep-logs",
				"--outdir",
				tempDir,
				texPath,
			},
			Runs: 1,
		},
		{
			Command: "xelatex",
			Args: []string{
				"-interaction=nonstopmode",
				"-halt-on-error",
				"-file-line-error",
				"-output-directory",
				tempDir,
				texFilename,
			},
			Runs: 2,
		},
		{
			Command: "pdflatex",
			Args: []string{
				"-interaction=nonstopmode",
				"-halt-on-error",
				"-file-line-error",
				"-output-directory",
				tempDir,
				texFilename,
			},
			Runs: 2,
		},
	}

	attempted := make([]string, 0, len(compilers))
	failures := make([]string, 0, len(compilers))
	for _, c := range compilers {
		if _, err := exec.LookPath(c.Command); err != nil {
			continue
		}

		attempted = append(attempted, c.Command)
		compilerSucceeded := true
		var lastOutput string
		for i := 0; i < c.Runs; i++ {
			cmd := exec.Command(c.Command, c.Args...)
			cmd.Dir = tempDir

			output, err := cmd.CombinedOutput()
			lastOutput = string(output)
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					failures = append(failures, fmt.Sprintf("%s: %s", c.Command, extractErrorMessage(lastOutput)))
					compilerSucceeded = false
					break
				}
				failures = append(failures, fmt.Sprintf("%s: failed to run command (%v)", c.Command, err))
				compilerSucceeded = false
				break
			}
		}

		if compilerSucceeded {
			return lastOutput, nil
		}
	}

	if len(attempted) == 0 {
		return "", errors.New("no LaTeX compiler found. Install TeX Live/MacTeX (pdflatex or xelatex), latexmk, or tectonic")
	}

	if len(failures) == 0 {
		return "", fmt.Errorf("LaTeX compilation failed after trying: %s", strings.Join(attempted, ", "))
	}

	return "", fmt.Errorf("LaTeX compilation failed after trying %s. %s", strings.Join(attempted, ", "), failures[0])
}

const pageCountMarker = "HARAESUME_PAGES:"

// CompileToSinglePagePDF compiles LaTeX and, if the result exceeds one page,
// progressively tightens spacing/margins and recompiles until it fits.
func CompileToSinglePagePDF(latexSource string) ([]byte, error) {
	probed := injectPageCountProbe(latexSource)
	pdf, compilerLog, err := compileToPDFInternal(probed)
	if err != nil {
		return nil, err
	}

	pages := extractPageCount(compilerLog)
	if pages <= 1 {
		return pdf, nil
	}

	log.Printf("Resume PDF has %d pages, applying spacing adjustments to fit single page", pages)

	for level := 1; level <= 2; level++ {
		tightened := injectPageCountProbe(tightenLatexSpacing(latexSource, level))
		tightenedPDF, tightenedLog, compileErr := compileToPDFInternal(tightened)
		if compileErr != nil {
			log.Printf("Tightening level %d failed to compile: %v", level, compileErr)
			continue
		}
		pages = extractPageCount(tightenedLog)
		if pages <= 1 {
			log.Printf("Tightening level %d succeeded: %d page(s)", level, pages)
			return tightenedPDF, nil
		}
		log.Printf("Tightening level %d still produces %d pages", level, pages)
		pdf = tightenedPDF
	}

	log.Printf("Could not reduce to single page after all tightening levels")
	return pdf, nil
}

var docclassFontRe = regexp.MustCompile(`(\\documentclass\[[^\]]*)11pt([^\]]*\])`)
var pageCountRe = regexp.MustCompile(pageCountMarker + `(\d+)`)

func injectPageCountProbe(source string) string {
	probe := `\AtEndDocument{\typeout{` + pageCountMarker + `\arabic{page}}}` + "\n"
	idx := strings.Index(source, `\begin{document}`)
	if idx < 0 {
		return source
	}
	return source[:idx] + probe + source[idx:]
}

func extractPageCount(compilerLog string) int {
	m := pageCountRe.FindStringSubmatch(compilerLog)
	if m == nil {
		return 1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 1
	}
	return n
}

func tightenLatexSpacing(source string, level int) string {
	result := source
	var overrides []string

	switch level {
	case 1:
		overrides = []string{
			`\addtolength{\topmargin}{-0.15in}`,
			`\addtolength{\textheight}{0.3in}`,
		}
	default:
		result = docclassFontRe.ReplaceAllString(result, "${1}10pt${2}")
		overrides = []string{
			`\addtolength{\topmargin}{-0.2in}`,
			`\addtolength{\textheight}{0.4in}`,
			`\addtolength{\oddsidemargin}{-0.05in}`,
			`\addtolength{\evensidemargin}{-0.05in}`,
			`\addtolength{\textwidth}{0.1in}`,
		}
	}

	block := "\n% haraesume: auto-fit single page\n" + strings.Join(overrides, "\n") + "\n"

	idx := strings.Index(result, `\begin{document}`)
	if idx >= 0 {
		result = result[:idx] + block + result[idx:]
	}

	return result
}

func extractErrorMessage(logOutput string) string {
	lines := strings.Split(logOutput, "\n")
	errorsFound := make([]string, 0, 5)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "!") || strings.Contains(line, "Error:") {
			errorsFound = append(errorsFound, line)
			if len(errorsFound) == 5 {
				break
			}
		}
	}

	if len(errorsFound) == 0 {
		tail := make([]string, 0, 5)
		for i := len(lines) - 1; i >= 0 && len(tail) < 5; i-- {
			line := strings.TrimSpace(lines[i])
			if line != "" {
				tail = append(tail, line)
			}
		}
		if len(tail) == 0 {
			return "LaTeX compilation failed. Check your document syntax."
		}

		for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
			tail[i], tail[j] = tail[j], tail[i]
		}
		return strings.Join(tail, "\n")
	}

	return strings.Join(errorsFound, "\n")
}
