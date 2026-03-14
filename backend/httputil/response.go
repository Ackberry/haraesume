package httputil

import (
	"encoding/json"
	"log"
	"net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type JobDescriptionRequest struct {
	JobDescription string `json:"job_description"`
}

type OptimizeResponse struct {
	OptimizedLatex string `json:"optimized_latex"`
	ChangesSummary string `json:"changes_summary"`
}

type PDFResponse struct {
	PDFBase64       string   `json:"pdf_base64"`
	Filename        string   `json:"filename"`
	CompanyName     string   `json:"company_name,omitempty"`
	FolderPath      string   `json:"folder_path,omitempty"`
	ResumePDFPath   string   `json:"resume_pdf_path,omitempty"`
	PDFWarnings     []string `json:"pdf_warnings,omitempty"`
	TexFilesDeleted bool     `json:"tex_files_deleted"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type ResumeStatusResponse struct {
	HasResume bool `json:"has_resume"`
	Length    int  `json:"length"`
}

type ApplicationPackageResponse struct {
	CompanyName     string   `json:"company_name"`
	FolderPath      string   `json:"folder_path"`
	ResumePDFPath   string   `json:"resume_pdf_path,omitempty"`
	OptimizedLatex  string   `json:"optimized_latex"`
	ChangesSummary  string   `json:"changes_summary"`
	PDFWarnings     []string `json:"pdf_warnings,omitempty"`
	TexFilesDeleted bool     `json:"tex_files_deleted"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

func WithCORS(next http.Handler) http.Handler {
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
