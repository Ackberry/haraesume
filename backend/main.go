package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	appVersion         = "0.1.0"
	serverAddr         = "0.0.0.0:3001"
	openRouterBaseURL  = "https://openrouter.ai/api/v1/chat/completions"
	openRouterModel    = "anthropic/claude-sonnet-4"
	maxMultipartMemory = 16 << 20
)

type appState struct {
	mu                    sync.RWMutex
	currentResume         *string
	currentJobDescription *string
}

func (s *appState) setResume(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := value
	s.currentResume = &v
}

func (s *appState) getResume() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentResume == nil {
		return "", false
	}
	return *s.currentResume, true
}

func (s *appState) setJobDescription(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := value
	s.currentJobDescription = &v
}

func (s *appState) getJobDescription() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentJobDescription == nil {
		return "", false
	}
	return *s.currentJobDescription, true
}

type server struct {
	state *appState
}

type errorResponse struct {
	Error string `json:"error"`
}

type jobDescriptionRequest struct {
	JobDescription string `json:"job_description"`
}

type optimizeResponse struct {
	OptimizedLatex string `json:"optimized_latex"`
	ChangesSummary string `json:"changes_summary"`
}

type coverLetterResponse struct {
	CoverLetter string `json:"cover_letter"`
}

type pdfResponse struct {
	PDFBase64 string `json:"pdf_base64"`
	Filename  string `json:"filename"`
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func main() {
	s := &server{
		state: &appState{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthCheck)
	mux.HandleFunc("/api/upload-resume", s.uploadResume)
	mux.HandleFunc("/api/job-description", s.setJobDescription)
	mux.HandleFunc("/api/optimize", s.optimizeResume)
	mux.HandleFunc("/api/cover-letter", s.generateCoverLetter)
	mux.HandleFunc("/api/generate-pdf", s.generatePDF)

	handler := withCORS(mux)
	log.Printf("Server running on http://localhost:3001")
	if err := http.ListenAndServe(serverAddr, handler); err != nil {
		log.Fatal(err)
	}
}

func (s *server) healthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "healthy",
		Version: appVersion,
	})
}

func (s *server) uploadResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read multipart: %v", err))
		return
	}

	file, _, err := r.FormFile("resume")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No resume file found in request")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	if !utf8.Valid(data) {
		writeError(w, http.StatusBadRequest, "File must be valid UTF-8 (LaTeX)")
		return
	}

	content := string(data)
	s.state.setResume(content)

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Resume uploaded successfully",
		"length":  len(content),
	})
}

func (s *server) setJobDescription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var payload jobDescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	s.state.setJobDescription(payload.JobDescription)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Job description saved",
	})
}

func (s *server) optimizeResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resume, ok := s.state.getResume()
	if !ok {
		writeError(w, http.StatusBadRequest, "No resume uploaded. Please upload a resume first.")
		return
	}

	jobDescription, ok := s.state.getJobDescription()
	if !ok {
		writeError(w, http.StatusBadRequest, "No job description provided. Please set a job description first.")
		return
	}

	optimizedLatex, changesSummary, err := optimizeResumeWithLLM(resume, jobDescription)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("LLM error: %v", err))
		return
	}

	s.state.setResume(optimizedLatex)
	writeJSON(w, http.StatusOK, optimizeResponse{
		OptimizedLatex: optimizedLatex,
		ChangesSummary: changesSummary,
	})
}

func (s *server) generateCoverLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resume, ok := s.state.getResume()
	if !ok {
		writeError(w, http.StatusBadRequest, "No resume uploaded")
		return
	}

	jobDescription, ok := s.state.getJobDescription()
	if !ok {
		writeError(w, http.StatusBadRequest, "No job description provided")
		return
	}

	coverLetter, err := generateCoverLetterWithLLM(resume, jobDescription)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("LLM error: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, coverLetterResponse{
		CoverLetter: coverLetter,
	})
}

func (s *server) generatePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resume, ok := s.state.getResume()
	if !ok {
		writeError(w, http.StatusBadRequest, "No resume to convert")
		return
	}

	pdfBytes, err := compileToPDF(resume)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("PDF generation failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, pdfResponse{
		PDFBase64: base64.StdEncoding.EncodeToString(pdfBytes),
		Filename:  "resume.pdf",
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func optimizeResumeWithLLM(resumeLatex, jobDescription string) (string, string, error) {
	systemPrompt := `You are an expert resume optimizer. Your task is to modify a LaTeX resume to better match a job description while:
1. Keeping all LaTeX syntax valid and compilable
2. Preserving the original structure and formatting
3. Tailoring bullet points to highlight relevant experience
4. Adding relevant keywords from the job description naturally
5. Quantifying achievements where possible
6. Ensuring ATS compatibility (no tables, simple formatting)

IMPORTANT: Return ONLY valid LaTeX code. Do not include markdown code fences or explanations in the LaTeX output.`

	userPrompt := fmt.Sprintf(`Please optimize this resume for the following job:

## Job Description:
%s

## Current Resume (LaTeX):
%s

Return the optimized LaTeX resume. After the LaTeX, add a separator "---CHANGES---" and briefly list the key changes you made.`, jobDescription, resumeLatex)

	content, err := runLLM(systemPrompt, userPrompt, 4096)
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(content, "---CHANGES---", 2)
	optimizedLatex := strings.TrimSpace(parts[0])
	changesSummary := ""
	if len(parts) > 1 {
		changesSummary = strings.TrimSpace(parts[1])
	}

	return optimizedLatex, changesSummary, nil
}

func generateCoverLetterWithLLM(resumeLatex, jobDescription string) (string, error) {
	systemPrompt := `You are an expert cover letter writer. Create a compelling, personalized cover letter that:
1. Highlights relevant experience from the resume
2. Shows genuine interest in the specific role and company
3. Connects the candidate's skills to job requirements
4. Uses a professional but engaging tone
5. Is concise (3-4 paragraphs, under 400 words)
6. Avoids generic phrases and cliches

Return only the cover letter text, ready to be used.`

	userPrompt := fmt.Sprintf(`Write a cover letter for this job application:

## Job Description:
%s

## Candidate's Resume (LaTeX, extract relevant info):
%s`, jobDescription, resumeLatex)

	content, err := runLLM(systemPrompt, userPrompt, 1024)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(content), nil
}

func runLLM(systemPrompt, userPrompt string, maxTokens int) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		return "", errors.New("missing API key: set OPENROUTER_API_KEY environment variable")
	}

	requestBody := chatRequest{
		Model: openRouterModel,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: maxTokens,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to serialize LLM request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, openRouterBaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read LLM response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return "", errors.New("no response from model")
	}

	content, err := parseMessageContent(parsed.Choices[0].Message.Content)
	if err != nil {
		return "", err
	}

	return content, nil
}

func parseMessageContent(content any) (string, error) {
	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		var chunks []string
		for _, item := range value {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, ok := m["text"].(string)
			if ok && strings.TrimSpace(text) != "" {
				chunks = append(chunks, text)
			}
		}
		joined := strings.TrimSpace(strings.Join(chunks, "\n"))
		if joined == "" {
			return "", errors.New("no response from model")
		}
		return joined, nil
	default:
		return "", errors.New("no response from model")
	}
}

func compileToPDF(latexSource string) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "resume_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	texPath := filepath.Join(tempDir, "resume.tex")
	pdfPath := filepath.Join(tempDir, "resume.pdf")

	if err := os.WriteFile(texPath, []byte(latexSource), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write LaTeX source: %w", err)
	}

	for i := 0; i < 2; i++ {
		if err := runPDFLatex(tempDir, texPath); err != nil {
			return nil, err
		}
	}

	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, errors.New("failed to read PDF output")
	}

	return pdfBytes, nil
}

func runPDFLatex(tempDir, texPath string) error {
	cmd := exec.Command(
		"pdflatex",
		"-interaction=nonstopmode",
		"-halt-on-error",
		"-output-directory",
		tempDir,
		texPath,
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return errors.New("pdflatex not found. Please install TeX Live.")
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("pdflatex compilation failed: %s", extractLatexErrorMessage(string(output)))
	}

	return fmt.Errorf("failed to run pdflatex: %w", err)
}

func extractLatexErrorMessage(logOutput string) string {
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
		return "LaTeX compilation failed. Check your document syntax."
	}

	return strings.Join(errorsFound, "\n")
}
