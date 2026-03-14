package main

import (
	"log"
	"net/http"
	"strings"

	"backend/auth"
	"backend/config"
	"backend/email"
	"backend/httputil"
	"backend/resume"
	"backend/waitlist"
)

func main() {
	config.LoadEnvironment()

	authValidator, err := auth.NewValidatorFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	resumeStoreDir := config.ResolveResumeStoreDir()
	resumeState := resume.NewState()
	resumeStorage := resume.NewStorage(resumeStoreDir)

	waitlistPath := config.ResolveWaitlistFilePath()
	wlStore, wlErr := waitlist.NewStore(waitlistPath)
	if wlErr != nil {
		log.Fatal(wlErr)
	}
	log.Printf("Waitlist store loaded from %s (%d entries)", waitlistPath, wlStore.Len())

	emailFrom := strings.TrimSpace(config.GetEnv("RESEND_FROM_EMAIL"))
	if emailFrom == "" {
		emailFrom = "onboarding@resend.dev"
	}
	mailer := email.NewSender(strings.TrimSpace(config.GetEnv("RESEND_API_KEY")), emailFrom)
	if mailer.IsConfigured() {
		log.Printf("Resend email client initialized (from: %s)", emailFrom)
	} else {
		log.Println("RESEND_API_KEY not set — email sending disabled")
	}

	rh := resume.NewHandler(resumeState, resumeStorage)
	wh := waitlist.NewHandler(wlStore, mailer)
	requireAuth := auth.RequireAuth(authValidator)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", rh.HealthCheck)
	mux.Handle("/api/resume-status", requireAuth(http.HandlerFunc(rh.ResumeStatus)))
	mux.Handle("/api/upload-resume", requireAuth(http.HandlerFunc(rh.UploadResume)))
	mux.Handle("/api/job-description", requireAuth(http.HandlerFunc(rh.SetJobDescription)))
	mux.Handle("/api/optimize", requireAuth(http.HandlerFunc(rh.OptimizeResume)))
	mux.Handle("/api/generate-application-package", requireAuth(http.HandlerFunc(rh.GenerateApplicationPackage)))
	mux.Handle("/api/generate-pdf", requireAuth(http.HandlerFunc(rh.GeneratePDF)))
	mux.HandleFunc("/api/waitlist/notify-signup", wh.NotifySignupHandler)
	mux.Handle("/api/waitlist/status", requireAuth(http.HandlerFunc(wh.StatusHandler)))
	mux.Handle("/api/admin/waitlist", requireAuth(http.HandlerFunc(wh.AdminListHandler)))
	mux.Handle("/api/admin/waitlist/update", requireAuth(http.HandlerFunc(wh.AdminUpdateHandler)))

	handler := httputil.WithCORS(mux)
	log.Printf("Server running on http://localhost:3001")
	if err := http.ListenAndServe(config.ServerAddr, handler); err != nil {
		log.Fatal(err)
	}
}
