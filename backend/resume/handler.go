package resume

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"backend/auth"
	"backend/config"
	"backend/httputil"
	"backend/latex"
	"backend/llm"
)

type Handler struct {
	state   *State
	storage *Storage
}

func NewHandler(state *State, storage *Storage) *Handler {
	return &Handler{state: state, storage: storage}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, httputil.HealthResponse{
		Status:  "healthy",
		Version: config.AppVersion,
	})
}

func (h *Handler) ResumeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := auth.RequestUserID(r)
	if err := h.storage.EnsureBaseResumeLoaded(r.Context(), h.state, userID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load persisted resume: %v", err))
		return
	}

	resume, ok := h.state.GetBaseResume(userID)
	if !ok {
		httputil.WriteJSON(w, http.StatusOK, httputil.ResumeStatusResponse{
			HasResume: false,
			Length:    0,
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.ResumeStatusResponse{
		HasResume: true,
		Length:    len(resume),
	})
}

func (h *Handler) UploadResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if err := r.ParseMultipartForm(config.MaxMultipartMemory); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read multipart: %v", err))
		return
	}

	file, fileHeader, err := r.FormFile("resume")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "No resume file found in request")
		return
	}
	defer file.Close()

	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".tex") {
		httputil.WriteError(w, http.StatusBadRequest, "Only .tex resume files are supported for upload")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	if !utf8.Valid(data) {
		httputil.WriteError(w, http.StatusBadRequest, "File must be valid UTF-8 (LaTeX)")
		return
	}

	if err := llm.ValidateInputLength(string(data), llm.MaxResumeBytes, "Resume"); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	content := llm.SanitizeUserInput(string(data))
	userID := auth.RequestUserID(r)
	h.state.SetBaseResume(userID, content)
	if err := h.storage.PersistBaseResume(r.Context(), userID, content); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to persist resume: %v", err))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Resume uploaded and saved successfully",
		"length":  len(content),
	})
}

func (h *Handler) SetJobDescription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var payload httputil.JobDescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	if err := llm.ValidateInputLength(payload.JobDescription, llm.MaxJobDescriptionBytes, "Job description"); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	sanitizedJD := llm.SanitizeUserInput(payload.JobDescription)
	userID := auth.RequestUserID(r)
	h.state.SetJobDescription(userID, sanitizedJD)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Job description saved",
	})
}

func (h *Handler) OptimizeResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := auth.RequestUserID(r)
	if err := h.storage.EnsureBaseResumeLoaded(r.Context(), h.state, userID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load persisted resume: %v", err))
		return
	}

	resume, ok := h.state.GetBaseResume(userID)
	if !ok {
		httputil.WriteError(w, http.StatusBadRequest, "No base resume found. Please upload a resume first.")
		return
	}

	jobDescription, ok := h.state.GetJobDescription(userID)
	if !ok {
		httputil.WriteError(w, http.StatusBadRequest, "No job description provided. Please set a job description first.")
		return
	}

	optimizedLatex, changesSummary, err := llm.OptimizeResume(resume, jobDescription)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("LLM error: %v", err))
		return
	}

	h.state.SetOptimizedResume(userID, optimizedLatex)
	httputil.WriteJSON(w, http.StatusOK, httputil.OptimizeResponse{
		OptimizedLatex: optimizedLatex,
		ChangesSummary: changesSummary,
	})
}

func (h *Handler) GenerateApplicationPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := auth.RequestUserID(r)
	if err := h.storage.EnsureBaseResumeLoaded(r.Context(), h.state, userID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load persisted resume: %v", err))
		return
	}

	resume, ok := h.state.GetBaseResume(userID)
	if !ok {
		httputil.WriteError(w, http.StatusBadRequest, "No base resume found. Please upload a resume first.")
		return
	}

	jobDescription, ok := h.state.GetJobDescription(userID)
	if !ok {
		httputil.WriteError(w, http.StatusBadRequest, "No job description provided. Please set a job description first.")
		return
	}

	optimizedLatex, changesSummary, err := llm.OptimizeResume(resume, jobDescription)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Resume generation failed: %v", err))
		return
	}

	h.state.SetOptimizedResume(userID, optimizedLatex)

	companyName, outputDir, err := prepareCompanyOutputDir(jobDescription, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to prepare output folder: %v", err))
		return
	}
	resumeBaseFilename := llm.BuildResumeBaseFilename(companyName)

	resumeTexPath := filepath.Join(outputDir, resumeBaseFilename+".tex")
	cleanupRan := false
	cleanupTexFiles := func() bool {
		return removeIfExists(resumeTexPath)
	}
	defer func() {
		if !cleanupRan {
			_ = cleanupTexFiles()
		}
	}()

	if err := os.WriteFile(resumeTexPath, []byte(optimizedLatex), 0o600); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save resume file: %v", err))
		return
	}

	response := httputil.ApplicationPackageResponse{
		CompanyName:    companyName,
		FolderPath:     safeAbsPath(outputDir),
		OptimizedLatex: optimizedLatex,
		ChangesSummary: changesSummary,
	}

	var pdfWarnings []string
	resumePDFPath := filepath.Join(outputDir, resumeBaseFilename+".pdf")
	resumePDF, err := latex.CompileToSinglePagePDF(optimizedLatex)
	if err != nil {
		pdfWarnings = append(pdfWarnings, "Resume PDF generation failed: "+err.Error())
	} else if err := os.WriteFile(resumePDFPath, resumePDF, 0o600); err != nil {
		pdfWarnings = append(pdfWarnings, "Resume PDF save failed: "+err.Error())
	} else {
		response.ResumePDFPath = safeAbsPath(resumePDFPath)
	}

	if len(pdfWarnings) > 0 {
		response.PDFWarnings = pdfWarnings
	}
	response.TexFilesDeleted = cleanupTexFiles()
	cleanupRan = true

	httputil.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) GeneratePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := auth.RequestUserID(r)
	if err := h.storage.EnsureBaseResumeLoaded(r.Context(), h.state, userID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load persisted resume: %v", err))
		return
	}

	resume, ok := h.state.GetActiveResume(userID)
	if !ok {
		httputil.WriteError(w, http.StatusBadRequest, "No resume to convert")
		return
	}

	pdfBytes, err := latex.CompileToSinglePagePDF(resume)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("PDF generation failed: %v", err))
		return
	}

	jobDescription, _ := h.state.GetJobDescription(userID)
	companyName, outputDir, err := prepareCompanyOutputDir(jobDescription, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to prepare output folder: %v", err))
		return
	}
	resumeBaseFilename := llm.BuildResumeBaseFilename(companyName)

	resumeTexPath := filepath.Join(outputDir, resumeBaseFilename+".tex")
	cleanupRan := false
	cleanupTexFile := func() bool {
		return removeIfExists(resumeTexPath)
	}
	defer func() {
		if !cleanupRan {
			_ = cleanupTexFile()
		}
	}()

	if err := os.WriteFile(resumeTexPath, []byte(resume), 0o600); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save resume file: %v", err))
		return
	}

	response := httputil.PDFResponse{
		PDFBase64:   base64.StdEncoding.EncodeToString(pdfBytes),
		Filename:    resumeBaseFilename + ".pdf",
		CompanyName: companyName,
		FolderPath:  safeAbsPath(outputDir),
	}

	resumePDFPath := filepath.Join(outputDir, resumeBaseFilename+".pdf")
	if err := os.WriteFile(resumePDFPath, pdfBytes, 0o600); err != nil {
		response.PDFWarnings = append(response.PDFWarnings, "Resume PDF save failed: "+err.Error())
	} else {
		response.ResumePDFPath = safeAbsPath(resumePDFPath)
	}

	response.TexFilesDeleted = cleanupTexFile()
	cleanupRan = true

	httputil.WriteJSON(w, http.StatusOK, response)
}

func prepareCompanyOutputDir(jobDescription, userID string) (companyName string, outputDir string, err error) {
	companyName = llm.ResolveCompanyName(jobDescription)
	companyDirName := llm.SanitizeFolderName(companyName)
	userDirName := UserStorageDirName(userID)

	applicationsRootDir, err := config.ResolveApplicationsRootDir()
	if err != nil {
		return "", "", err
	}

	outputDir = filepath.Join(applicationsRootDir, userDirName, companyDirName)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", "", err
	}
	return companyName, outputDir, nil
}

func safeAbsPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolute
}

func removeIfExists(path string) bool {
	err := os.Remove(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	log.Printf("failed to remove file %s: %v", path, err)
	return false
}
