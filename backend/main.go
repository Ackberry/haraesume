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
	appVersion                 = "0.1.0"
	serverAddr                 = "0.0.0.0:3001"
	openRouterBaseURL          = "https://openrouter.ai/api/v1/chat/completions"
	openRouterModel            = "anthropic/claude-sonnet-4"
	maxMultipartMemory         = 16 << 20
	defaultResumePath          = "state/base_resume.tex"
	defaultApplicationsRootDir = "state/applications"
	resumeOutputFilename       = "Akbari, Deep"
	coverLetterOutputFilename  = "CV_Deep"
)

type appState struct {
	mu                    sync.RWMutex
	baseResume            *string
	currentOptimized      *string
	currentCoverLetter    *string
	currentJobDescription *string
}

func (s *appState) setBaseResume(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := value
	s.baseResume = &v
	s.currentOptimized = nil
	s.currentCoverLetter = nil
}

func (s *appState) setOptimizedResume(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := value
	s.currentOptimized = &v
}

func (s *appState) setCoverLetter(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := value
	s.currentCoverLetter = &v
}

func (s *appState) getBaseResume() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.baseResume == nil {
		return "", false
	}
	return *s.baseResume, true
}

func (s *appState) getActiveResume() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentOptimized != nil {
		return *s.currentOptimized, true
	}
	if s.baseResume != nil {
		return *s.baseResume, true
	}
	return "", false
}

func (s *appState) setJobDescription(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := value
	s.currentJobDescription = &v
	s.currentOptimized = nil
	s.currentCoverLetter = nil
}

func (s *appState) getJobDescription() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentJobDescription == nil {
		return "", false
	}
	return *s.currentJobDescription, true
}

func (s *appState) getCoverLetter() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentCoverLetter == nil {
		return "", false
	}
	return *s.currentCoverLetter, true
}

type server struct {
	state           *appState
	resumeStorePath string
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
	CoverLetter      string `json:"cover_letter"`
	CoverLetterLatex string `json:"cover_letter_latex"`
}

type pdfResponse struct {
	PDFBase64 string `json:"pdf_base64"`
	Filename  string `json:"filename"`
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type resumeStatusResponse struct {
	HasResume bool `json:"has_resume"`
	Length    int  `json:"length"`
}

type applicationPackageResponse struct {
	CompanyName        string   `json:"company_name"`
	FolderPath         string   `json:"folder_path"`
	ResumeTexPath      string   `json:"resume_tex_path"`
	ResumePDFPath      string   `json:"resume_pdf_path,omitempty"`
	CoverLetterTex     string   `json:"cover_letter_latex"`
	CoverLetterTexPath string   `json:"cover_letter_tex_path"`
	CoverLetterPDFPath string   `json:"cover_letter_pdf_path,omitempty"`
	OptimizedLatex     string   `json:"optimized_latex"`
	ChangesSummary     string   `json:"changes_summary"`
	PDFWarnings        []string `json:"pdf_warnings,omitempty"`
}

func main() {
	loadEnvironment()

	resumeStorePath := strings.TrimSpace(os.Getenv("RESUME_STORE_PATH"))
	if resumeStorePath == "" {
		resumeStorePath = defaultResumePath
	}

	s := &server{
		state:           &appState{},
		resumeStorePath: resumeStorePath,
	}
	if err := s.loadPersistedBaseResume(); err != nil {
		log.Printf("Unable to load persisted resume: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthCheck)
	mux.HandleFunc("/api/resume-status", s.resumeStatus)
	mux.HandleFunc("/api/upload-resume", s.uploadResume)
	mux.HandleFunc("/api/job-description", s.setJobDescription)
	mux.HandleFunc("/api/optimize", s.optimizeResume)
	mux.HandleFunc("/api/cover-letter", s.generateCoverLetter)
	mux.HandleFunc("/api/generate-application-package", s.generateApplicationPackage)
	mux.HandleFunc("/api/generate-cover-letter-pdf", s.generateCoverLetterPDF)
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

func (s *server) loadPersistedBaseResume() error {
	data, err := os.ReadFile(s.resumeStorePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !utf8.Valid(data) {
		return errors.New("persisted resume is not valid UTF-8")
	}
	content := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	if content == "" {
		return nil
	}
	s.state.setBaseResume(content)
	log.Printf("Loaded persisted base resume (%d chars)", len(content))
	return nil
}

func (s *server) persistBaseResume(content string) error {
	dir := filepath.Dir(s.resumeStorePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.resumeStorePath, []byte(content), 0o600)
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

func (s *server) resumeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resume, ok := s.state.getBaseResume()
	if !ok {
		writeJSON(w, http.StatusOK, resumeStatusResponse{
			HasResume: false,
			Length:    0,
		})
		return
	}

	writeJSON(w, http.StatusOK, resumeStatusResponse{
		HasResume: true,
		Length:    len(resume),
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
	s.state.setBaseResume(content)
	if err := s.persistBaseResume(content); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to persist resume: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Resume uploaded and saved successfully",
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

	resume, ok := s.state.getBaseResume()
	if !ok {
		writeError(w, http.StatusBadRequest, "No base resume found. Please upload a resume first.")
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

	s.state.setOptimizedResume(optimizedLatex)
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

	resume, ok := s.state.getBaseResume()
	if !ok {
		writeError(w, http.StatusBadRequest, "No base resume found")
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
	s.state.setCoverLetter(coverLetter)

	writeJSON(w, http.StatusOK, coverLetterResponse{
		CoverLetter:      coverLetter,
		CoverLetterLatex: coverLetter,
	})
}

func (s *server) generateApplicationPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resume, ok := s.state.getBaseResume()
	if !ok {
		writeError(w, http.StatusBadRequest, "No base resume found. Please upload a resume first.")
		return
	}

	jobDescription, ok := s.state.getJobDescription()
	if !ok {
		writeError(w, http.StatusBadRequest, "No job description provided. Please set a job description first.")
		return
	}

	optimizedLatex, changesSummary, err := optimizeResumeWithLLM(resume, jobDescription)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Resume generation failed: %v", err))
		return
	}

	coverLetterLatex, err := generateCoverLetterWithLLM(optimizedLatex, jobDescription)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Cover letter generation failed: %v", err))
		return
	}

	s.state.setOptimizedResume(optimizedLatex)
	s.state.setCoverLetter(coverLetterLatex)

	companyName := extractCompanyName(jobDescription)
	companyDirName := sanitizeFolderName(companyName)
	outputDir := filepath.Join(defaultApplicationsRootDir, companyDirName)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create output folder: %v", err))
		return
	}

	resumeTexPath := filepath.Join(outputDir, resumeOutputFilename+".tex")
	coverLetterTexPath := filepath.Join(outputDir, coverLetterOutputFilename+".tex")
	if err := os.WriteFile(resumeTexPath, []byte(optimizedLatex), 0o600); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save resume file: %v", err))
		return
	}
	if err := os.WriteFile(coverLetterTexPath, []byte(coverLetterLatex), 0o600); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save cover letter file: %v", err))
		return
	}

	response := applicationPackageResponse{
		CompanyName:        companyName,
		FolderPath:         safeAbsPath(outputDir),
		ResumeTexPath:      safeAbsPath(resumeTexPath),
		CoverLetterTex:     coverLetterLatex,
		CoverLetterTexPath: safeAbsPath(coverLetterTexPath),
		OptimizedLatex:     optimizedLatex,
		ChangesSummary:     changesSummary,
	}

	var pdfWarnings []string
	resumePDFPath := filepath.Join(outputDir, resumeOutputFilename+".pdf")
	resumePDF, err := compileToPDF(optimizedLatex)
	if err != nil {
		pdfWarnings = append(pdfWarnings, "Resume PDF generation failed: "+err.Error())
	} else if err := os.WriteFile(resumePDFPath, resumePDF, 0o600); err != nil {
		pdfWarnings = append(pdfWarnings, "Resume PDF save failed: "+err.Error())
	} else {
		response.ResumePDFPath = safeAbsPath(resumePDFPath)
	}

	coverLetterPDFPath := filepath.Join(outputDir, coverLetterOutputFilename+".pdf")
	coverLetterPDF, err := compileToPDF(coverLetterLatex)
	if err != nil {
		pdfWarnings = append(pdfWarnings, "Cover letter PDF generation failed: "+err.Error())
	} else if err := os.WriteFile(coverLetterPDFPath, coverLetterPDF, 0o600); err != nil {
		pdfWarnings = append(pdfWarnings, "Cover letter PDF save failed: "+err.Error())
	} else {
		response.CoverLetterPDFPath = safeAbsPath(coverLetterPDFPath)
	}

	if len(pdfWarnings) > 0 {
		response.PDFWarnings = pdfWarnings
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *server) generateCoverLetterPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	coverLetterLatex, ok := s.state.getCoverLetter()
	if !ok {
		writeError(w, http.StatusBadRequest, "No generated cover letter found. Generate one first.")
		return
	}

	pdfBytes, err := compileToPDF(coverLetterLatex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("PDF generation failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, pdfResponse{
		PDFBase64: base64.StdEncoding.EncodeToString(pdfBytes),
		Filename:  "cover_letter.pdf",
	})
}

func (s *server) generatePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resume, ok := s.state.getActiveResume()
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

func extractCompanyName(jobDescription string) string {
	jobDescription = strings.TrimSpace(strings.TrimPrefix(jobDescription, "\ufeff"))
	if jobDescription == "" {
		return "Unknown Company"
	}

	for _, pattern := range []*regexp.Regexp{
		companyLabelRegex,
		companyJoinRegex,
		companyAtRegex,
		companyIsRegex,
	} {
		matches := pattern.FindStringSubmatch(jobDescription)
		if len(matches) < 2 {
			continue
		}
		company := normalizeCompanyName(matches[1])
		if company != "" {
			return company
		}
	}

	lines := strings.Split(jobDescription, "\n")
	maxLines := minInt(12, len(lines))
	for i := 0; i < maxLines; i++ {
		line := strings.TrimSpace(strings.TrimLeft(lines[i], "-*• \t"))
		company := normalizeCompanyName(line)
		if company == "" {
			continue
		}
		if looksLikeCompanyName(company) {
			return company
		}
	}

	return "Unknown Company"
}

func normalizeCompanyName(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	for _, sep := range []string{" | ", " - ", " — ", " – ", ":"} {
		if idx := strings.Index(value, sep); idx > 0 {
			value = value[:idx]
		}
	}

	value = strings.Trim(value, "\"'`.,;:|()[]{} ")
	value = strings.TrimSpace(spaceRegex.ReplaceAllString(value, " "))
	if value == "" {
		return ""
	}
	if len(value) > 100 {
		value = strings.TrimSpace(value[:100])
	}

	return value
}

func looksLikeCompanyName(value string) bool {
	lower := strings.ToLower(value)
	if lower == "" {
		return false
	}

	for _, badTerm := range []string{
		"responsibilities",
		"requirements",
		"qualification",
		"job description",
		"position",
		"benefits",
		"salary",
		"location",
		"remote",
		"full-time",
		"full time",
		"internship",
	} {
		if strings.Contains(lower, badTerm) {
			return false
		}
	}

	companyHints := []string{
		"inc",
		"llc",
		"corp",
		"corporation",
		"technologies",
		"technology",
		"systems",
		"labs",
		"group",
		"company",
		"solutions",
	}
	if containsAny(lower, companyHints) {
		return true
	}

	words := strings.Fields(value)
	if len(words) == 0 || len(words) > 8 {
		return false
	}

	capitalized := 0
	for _, word := range words {
		trimmed := strings.Trim(word, ".,;:()[]{}")
		if trimmed == "" {
			continue
		}
		r, _ := utf8.DecodeRuneInString(trimmed)
		if r >= 'A' && r <= 'Z' {
			capitalized++
		}
	}
	return capitalized >= len(words)-1
}

func sanitizeFolderName(name string) string {
	cleaned := normalizeCompanyName(name)
	if cleaned == "" || strings.EqualFold(cleaned, "unknown company") {
		return "Unknown Company"
	}

	cleaned = invalidPathCharRegex.ReplaceAllString(cleaned, "")
	cleaned = strings.TrimSpace(spaceRegex.ReplaceAllString(cleaned, " "))
	cleaned = strings.Trim(cleaned, ". ")
	if cleaned == "" {
		return "Unknown Company"
	}
	return cleaned
}

func safeAbsPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolute
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
	const maxSkillAdds = 5
	targetedSkills := suggestMissingTechnicalSkills(resumeLatex, jobDescription, maxSkillAdds)

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

	content, err := runLLM(systemPrompt, userPrompt, 4096)
	if err != nil {
		return "", "", err
	}

	optimizedLatex, changesSummary, err := parseOptimizationOutput(content)
	if err != nil {
		return "", "", err
	}

	optimizedLatex = restoreLockedSections(resumeLatex, optimizedLatex, []string{
		"experience",
		"projects",
		"leadership",
	})

	return optimizedLatex, changesSummary, nil
}

var (
	latexFenceRegex   = regexp.MustCompile("(?is)```(?:latex)?\\s*(.*?)```")
	latexCommentRegex = regexp.MustCompile(`(?m)%.*$`)
	latexCommandRegex = regexp.MustCompile(`\\[A-Za-z]+[*]?`)
	sectionHeaderRE   = regexp.MustCompile(`(?m)^\s*\\section\{([^}]+)\}`)
	spaceRegex        = regexp.MustCompile(`[ \t]+`)
	numberRegex       = regexp.MustCompile(`\b\d+(?:[.,]\d+)?(?:%|x|k|m|ms)?\b`)

	companyLabelRegex = regexp.MustCompile(`(?im)^\s*(?:company|organization|employer)\s*[:\-]\s*([A-Z][A-Za-z0-9&.,'() \-]{1,100})\s*$`)
	companyJoinRegex  = regexp.MustCompile(`(?is)\bjoin\s+([A-Z][A-Za-z0-9&.,'() \-]{1,80}?)(?:\s+(?:as|to|for|where|and)\b|[,\n.])`)
	companyAtRegex    = regexp.MustCompile(`(?is)\bat\s+([A-Z][A-Za-z0-9&.,'() \-]{1,80}?)(?:\s+(?:as|to|for|where|and|is|are)\b|[,\n.])`)
	companyIsRegex    = regexp.MustCompile(`(?is)\b([A-Z][A-Za-z0-9&.,'() \-]{1,80}?)\s+is\s+(?:a|an|the)\b`)

	invalidPathCharRegex = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
)

type skillCandidate struct {
	Name     string
	Category string
	Keywords []string
	Priority int
}

type skillSuggestion struct {
	Name     string
	Category string
	Score    int
}

var curatedSkillCandidates = []skillCandidate{
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

func suggestMissingTechnicalSkills(resumeLatex, jobDescription string, maxItems int) []skillSuggestion {
	if maxItems <= 0 {
		return nil
	}
	jobLower := strings.ToLower(jobDescription)
	techSection := strings.ToLower(getSectionContent(resumeLatex, "technical skills"))
	if techSection == "" {
		techSection = strings.ToLower(resumeLatex)
	}

	candidates := make([]skillSuggestion, 0, len(curatedSkillCandidates))
	for _, candidate := range curatedSkillCandidates {
		if resumeAlreadyHasSkill(techSection, candidate) {
			continue
		}
		matchCount := countSkillMentions(jobLower, candidate.Keywords)
		if matchCount == 0 {
			continue
		}
		score := candidate.Priority + (matchCount * 3) + emphasisBoost(jobLower, candidate.Keywords)
		candidates = append(candidates, skillSuggestion{
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
	out := make([]skillSuggestion, 0, minInt(maxItems, len(candidates)))
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

func formatSkillSuggestions(suggestions []skillSuggestion) string {
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
	lines := splitInformativeLines(jobLower)
	boost := 0
	for _, line := range lines {
		if !containsAny(line, []string{"required", "must", "minimum", "preferred", "qualification", "responsibilit"}) {
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

func resumeAlreadyHasSkill(technicalSectionLower string, candidate skillCandidate) bool {
	for _, keyword := range candidate.Keywords {
		if strings.Contains(technicalSectionLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

type latexSection struct {
	Title   string
	Start   int
	End     int
	Content string
}

func parseLatexSections(latex string) []latexSection {
	matches := sectionHeaderRE.FindAllStringSubmatchIndex(latex, -1)
	if len(matches) == 0 {
		return nil
	}

	sections := make([]latexSection, 0, len(matches))
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}
		start := match[0]
		end := len(latex)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		title := strings.TrimSpace(latex[match[2]:match[3]])
		sections = append(sections, latexSection{
			Title:   title,
			Start:   start,
			End:     end,
			Content: latex[start:end],
		})
	}
	return sections
}

func getSectionContent(latex string, sectionName string) string {
	target := normalizeSectionName(sectionName)
	for _, section := range parseLatexSections(latex) {
		if normalizeSectionName(section.Title) == target {
			return section.Content
		}
	}
	return ""
}

func restoreLockedSections(originalLatex, optimizedLatex string, lockedSections []string) string {
	originalSections := parseLatexSections(originalLatex)
	optimizedSections := parseLatexSections(optimizedLatex)
	if len(originalSections) == 0 || len(optimizedSections) == 0 {
		return optimizedLatex
	}

	locked := map[string]struct{}{}
	for _, name := range lockedSections {
		locked[normalizeSectionName(name)] = struct{}{}
	}

	originalByName := map[string]string{}
	for _, section := range originalSections {
		originalByName[normalizeSectionName(section.Title)] = section.Content
	}

	var builder strings.Builder
	prev := 0
	for _, section := range optimizedSections {
		builder.WriteString(optimizedLatex[prev:section.Start])
		sectionKey := normalizeSectionName(section.Title)
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

func normalizeSectionName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
		truncateForPrompt(jobDescription, 5000),
		formatBullets(jobHighlights),
		formatBullets(resumeHighlights),
		truncateForPrompt(resumePlain, 6000),
	)

	draftTemp := 0.65
	draft, err := runLLMWithTemperature(systemPrompt, userPrompt, 2200, &draftTemp)
	if err != nil {
		return "", err
	}
	draftLatex, err := parseCoverLetterLatexOutput(draft)
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
		truncateForPrompt(jobDescription, 5000),
		formatBullets(jobHighlights),
		formatBullets(resumeHighlights),
		truncateForPrompt(draftLatex, 7000),
	)

	refineTemp := 0.35
	refined, err := runLLMWithTemperature(refineSystemPrompt, refinePrompt, 2200, &refineTemp)
	if err != nil {
		return draftLatex, nil
	}

	finalLatex, err := parseCoverLetterLatexOutput(refined)
	if err != nil {
		return draftLatex, nil
	}
	return finalLatex, nil
}

func parseCoverLetterLatexOutput(content string) (string, error) {
	latex := extractLatexDocument(content)
	if latex == "" {
		return "", errors.New("model output did not contain a complete LaTeX cover letter document")
	}
	return latex, nil
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
