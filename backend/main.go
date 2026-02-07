package main

import (
	"bufio"
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
	"regexp"
	"sort"
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
	loadEnvironment()

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

func loadEnvironment() {
	candidates := []string{".env", filepath.Join("..", ".env")}
	for _, path := range candidates {
		if err := loadEnvFile(path); err == nil {
			log.Printf("Loaded environment from %s", path)
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			log.Printf("Skipping %s: %v", path, err)
		}
	}
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			isQuoted := (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')
			if isQuoted {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
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

	file, fileHeader, err := r.FormFile("resume")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No resume file found in request")
		return
	}
	defer file.Close()

	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".tex") {
		writeError(w, http.StatusBadRequest, "Only .tex resume files are supported for upload")
		return
	}

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
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature *float64      `json:"temperature,omitempty"`
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

	optimizedLatex, changesSummary, err := parseOptimizationOutput(content)
	if err != nil {
		return "", "", err
	}

	return optimizedLatex, changesSummary, nil
}

var (
	latexFenceRegex   = regexp.MustCompile("(?is)```(?:latex)?\\s*(.*?)```")
	latexCommentRegex = regexp.MustCompile(`(?m)%.*$`)
	latexCommandRegex = regexp.MustCompile(`\\[A-Za-z]+[*]?`)
	spaceRegex        = regexp.MustCompile(`[ \t]+`)
	numberRegex       = regexp.MustCompile(`\b\d+(?:[.,]\d+)?(?:%|x|k|m|ms)?\b`)
)

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

	optimizedLatex := extractLatexDocument(latexSegment)
	if optimizedLatex == "" {
		optimizedLatex = extractLatexDocument(raw)
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

func extractLatexDocument(content string) string {
	cleaned := strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	if cleaned == "" {
		return ""
	}

	fencedMatches := latexFenceRegex.FindAllStringSubmatch(cleaned, -1)
	for _, match := range fencedMatches {
		if len(match) < 2 {
			continue
		}
		if doc := trimToLatexDocument(match[1]); doc != "" {
			return doc
		}
	}

	return trimToLatexDocument(cleaned)
}

func trimToLatexDocument(content string) string {
	start := strings.Index(content, "\\documentclass")
	end := strings.LastIndex(content, "\\end{document}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	end += len("\\end{document}")
	return strings.TrimSpace(content[start:end])
}

func generateCoverLetterWithLLM(resumeLatex, jobDescription string) (string, error) {
	resumePlain := latexToPlainText(resumeLatex)
	resumeHighlights := pickResumeHighlights(resumePlain, 8)
	jobHighlights := pickJobHighlights(jobDescription, 8)

	if len(resumeHighlights) == 0 {
		resumeHighlights = []string{"No clear resume bullet highlights were extracted; use only verified input details."}
	}
	if len(jobHighlights) == 0 {
		jobHighlights = []string{"Prioritize the role's strongest requirement and align to relevant candidate evidence."}
	}

	systemPrompt := `You are an expert cover letter writer and editor.

Write in a natural human voice, not a template. Use these constraints:
- Be concrete and specific; avoid vague claims.
- Vary sentence rhythm and sentence openings. Mix short and longer sentences naturally.
- Keep first-person voice credible and professional.
- Align directly to this exact role and company details found in the job description.
- Use measured confidence and avoid hype.
- Never invent facts not present in the provided inputs.

Hard requirements:
1. 3-4 paragraphs, 220-380 words total.
2. Include at least two concrete examples from the resume highlights (metrics, outcomes, tools, or scope when available).
3. Address at least two job priorities from the job description.
4. Avoid cliches and stock phrases, including:
   "I am excited to apply", "I believe I am a great fit", "passionate about", "team player", "fast-paced environment", "think outside the box".
5. Avoid repetitive sentence starts (for example, do not begin most sentences with "I").

Return only the final cover letter text.`

	userPrompt := fmt.Sprintf(`Write a tailored cover letter using only the factual context below.

Job description:
%s

Top job priorities:
%s

Resume highlights:
%s

Resume plaintext context (for fact-checking):
%s`,
		truncateForPrompt(jobDescription, 5000),
		formatBullets(jobHighlights),
		formatBullets(resumeHighlights),
		truncateForPrompt(resumePlain, 6000),
	)

	draftTemp := 0.72
	draft, err := runLLMWithTemperature(systemPrompt, userPrompt, 1200, &draftTemp)
	if err != nil {
		return "", err
	}

	refineSystemPrompt := `You are a senior writing editor improving human-likeness and persuasiveness in cover letters.

Evaluate and revise the draft using this rubric:
1. Specificity and concreteness
2. Alignment to job requirements
3. Sentence rhythm and variety
4. Authentic professional voice
5. Cliche avoidance

If any area is weak, rewrite to fix it while preserving factual accuracy.

Keep the final letter:
- 3-4 paragraphs
- under 380 words
- professional, human, and specific
- free of markdown, headings, or lists

Return only the revised cover letter text.`

	refinePrompt := fmt.Sprintf(`Job description:
%s

Top job priorities:
%s

Resume highlights:
%s

Draft to improve:
%s`,
		truncateForPrompt(jobDescription, 5000),
		formatBullets(jobHighlights),
		formatBullets(resumeHighlights),
		truncateForPrompt(draft, 5000),
	)

	refineTemp := 0.45
	refined, err := runLLMWithTemperature(refineSystemPrompt, refinePrompt, 1200, &refineTemp)
	if err != nil {
		return cleanCoverLetterOutput(draft), nil
	}

	final := cleanCoverLetterOutput(refined)
	if final == "" {
		final = cleanCoverLetterOutput(draft)
	}
	return final, nil
}

func runLLM(systemPrompt, userPrompt string, maxTokens int) (string, error) {
	return runLLMWithTemperature(systemPrompt, userPrompt, maxTokens, nil)
}

func runLLMWithTemperature(systemPrompt, userPrompt string, maxTokens int, temperature *float64) (string, error) {
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
		MaxTokens:   maxTokens,
		Temperature: temperature,
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

func latexToPlainText(latex string) string {
	cleaned := latexCommentRegex.ReplaceAllString(latex, "")
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
	cleaned = latexCommandRegex.ReplaceAllString(cleaned, " ")

	lines := strings.Split(cleaned, "\n")
	output := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(spaceRegex.ReplaceAllString(line, " "))
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

func pickResumeHighlights(resumeText string, maxItems int) []string {
	lines := splitInformativeLines(resumeText)
	scored := scoreLines(lines, []string{
		"built", "developed", "designed", "led", "improved", "reduced", "increased",
		"launched", "shipped", "optimized", "automated", "implemented",
	}, []string{
		"python", "go", "java", "javascript", "typescript", "react", "node", "sql",
		"aws", "gcp", "azure", "docker", "kubernetes", "llm", "langchain", "langgraph",
	})
	return topScoredLines(scored, maxItems)
}

func pickJobHighlights(jobDescription string, maxItems int) []string {
	lines := splitInformativeLines(jobDescription)
	scored := scoreLines(lines, []string{
		"required", "must", "minimum", "preferred", "responsible", "responsibilities",
		"qualifications", "experience", "skills", "ability", "expect",
	}, []string{
		"python", "go", "java", "javascript", "typescript", "react", "node", "sql",
		"aws", "gcp", "azure", "docker", "kubernetes", "llm", "langchain", "langgraph",
	})
	return topScoredLines(scored, maxItems)
}

type scoredLine struct {
	text  string
	score int
}

func splitInformativeLines(text string) []string {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.TrimSpace(strings.TrimLeft(raw, "-*• \t"))
		line = strings.TrimSpace(spaceRegex.ReplaceAllString(line, " "))
		if len(line) < 20 {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func scoreLines(lines []string, priorityTerms []string, skillTerms []string) []scoredLine {
	seen := map[string]struct{}{}
	scored := make([]scoredLine, 0, len(lines))
	for _, line := range lines {
		normalized := strings.ToLower(line)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}

		score := 1
		if numberRegex.MatchString(normalized) {
			score += 3
		}
		if containsAny(normalized, priorityTerms) {
			score += 2
		}
		if containsAny(normalized, skillTerms) {
			score++
		}
		if len(strings.Fields(normalized)) >= 18 {
			score++
		}
		scored = append(scored, scoredLine{text: line, score: score})
	}
	return scored
}

func topScoredLines(lines []scoredLine, maxItems int) []string {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].score == lines[j].score {
			return len(lines[i].text) > len(lines[j].text)
		}
		return lines[i].score > lines[j].score
	})

	if maxItems <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) > maxItems {
		lines = lines[:maxItems]
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, line.text)
	}
	return out
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func formatBullets(items []string) string {
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

func truncateForPrompt(text string, maxRunes int) string {
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

func cleanCoverLetterOutput(text string) string {
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		if len(lines) >= 3 {
			lines = lines[1:]
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			cleaned = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	for _, prefix := range []string{"cover letter:", "final cover letter:"} {
		lower := strings.ToLower(cleaned)
		if strings.HasPrefix(lower, prefix) {
			cleaned = strings.TrimSpace(cleaned[len(prefix):])
		}
	}
	return cleaned
}

func compileToPDF(latexSource string) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "resume_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	latexSource = strings.TrimSpace(strings.TrimPrefix(latexSource, "\ufeff"))
	if normalized := extractLatexDocument(latexSource); normalized != "" {
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

	if err := runLatexCompiler(tempDir, texPath); err != nil {
		return nil, err
	}

	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, errors.New("failed to read PDF output")
	}

	return pdfBytes, nil
}

type latexCompiler struct {
	Command string
	Args    []string
	Runs    int
}

func runLatexCompiler(tempDir, texPath string) error {
	texFilename := filepath.Base(texPath)
	compilers := []latexCompiler{
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
	for _, compiler := range compilers {
		if _, err := exec.LookPath(compiler.Command); err != nil {
			continue
		}

		attempted = append(attempted, compiler.Command)
		compilerSucceeded := true
		for i := 0; i < compiler.Runs; i++ {
			cmd := exec.Command(compiler.Command, compiler.Args...)
			cmd.Dir = tempDir

			output, err := cmd.CombinedOutput()
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					failures = append(failures, fmt.Sprintf("%s: %s", compiler.Command, extractLatexErrorMessage(string(output))))
					compilerSucceeded = false
					break
				}
				failures = append(failures, fmt.Sprintf("%s: failed to run command (%v)", compiler.Command, err))
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
