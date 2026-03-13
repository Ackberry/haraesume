package latex

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type compiler struct {
	Command string
	Args    []string
	Runs    int
}

func CompileToPDF(latexSource string) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "resume_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	latexSource = strings.TrimSpace(strings.TrimPrefix(latexSource, "\ufeff"))
	if normalized := ExtractDocument(latexSource); normalized != "" {
		latexSource = normalized
	}
	if latexSource == "" {
		return nil, errors.New("no LaTeX source to compile")
	}

	texPath := filepath.Join(tempDir, "resume.tex")
	pdfPath := filepath.Join(tempDir, "resume.pdf")

	if err := os.WriteFile(texPath, []byte(latexSource), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write LaTeX source: %w", err)
	}

	if err := runCompiler(tempDir, texPath); err != nil {
		return nil, err
	}

	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, errors.New("failed to read PDF output")
	}

	return pdfBytes, nil
}

func runCompiler(tempDir, texPath string) error {
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
		for i := 0; i < c.Runs; i++ {
			cmd := exec.Command(c.Command, c.Args...)
			cmd.Dir = tempDir

			output, err := cmd.CombinedOutput()
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					failures = append(failures, fmt.Sprintf("%s: %s", c.Command, extractErrorMessage(string(output))))
					compilerSucceeded = false
					break
				}
				failures = append(failures, fmt.Sprintf("%s: failed to run command (%v)", c.Command, err))
				compilerSucceeded = false
				break
			}
		}

		if compilerSucceeded {
			return nil
		}
	}

	if len(attempted) == 0 {
		return errors.New("no LaTeX compiler found. Install TeX Live/MacTeX (pdflatex or xelatex), latexmk, or tectonic")
	}

	if len(failures) == 0 {
		return fmt.Errorf("LaTeX compilation failed after trying: %s", strings.Join(attempted, ", "))
	}

	return fmt.Errorf("LaTeX compilation failed after trying %s. %s", strings.Join(attempted, ", "), failures[0])
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
