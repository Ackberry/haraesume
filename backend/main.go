package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"

	"backend/auth"
	"backend/builder"
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

	// Pre-warm JWKS in background so it doesn't block server startup.
	if authValidator != nil {
		go func() {
			jwksCtx, jwksCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer jwksCancel()
			if err := authValidator.WarmUp(jwksCtx); err != nil {
				log.Printf("JWKS pre-warm failed (will retry on first request): %v", err)
			} else {
				log.Println("JWKS keys pre-warmed")
			}
		}()
	}

	projectID := strings.TrimSpace(config.GetEnv("FIREBASE_PROJECT_ID"))
	if projectID == "" {
		log.Fatal("FIREBASE_PROJECT_ID is required for Firestore")
	}
	fsCtx, fsCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer fsCancel()
	fsClient, err := firestore.NewClient(fsCtx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer fsClient.Close()

	// Verify Firestore connectivity at startup so we fail fast.
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	iter := fsClient.Collection("resumes").Limit(1).Documents(pingCtx)
	_, _ = iter.Next()
	iter.Stop()
	log.Println("Firestore connectivity verified")

	resumeState := resume.NewState()
	resumeStorage := resume.NewStorage(fsClient)

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
	bh := builder.NewHandler()
	requireAuth := auth.RequireAuth(authValidator)
	llmLimiter := httputil.NewRateLimiter(1*time.Minute, auth.RequestUserID)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", rh.HealthCheck)
	mux.Handle("/api/resume-status", requireAuth(http.HandlerFunc(rh.ResumeStatus)))
	mux.Handle("/api/upload-resume", requireAuth(http.HandlerFunc(rh.UploadResume)))
	mux.Handle("/api/job-description", requireAuth(http.HandlerFunc(rh.SetJobDescription)))
	mux.Handle("/api/optimize",
		requireAuth(httputil.WithRateLimit(llmLimiter, http.HandlerFunc(rh.OptimizeResume))))
	mux.Handle("/api/generate-application-package",
		requireAuth(httputil.WithRateLimit(llmLimiter, http.HandlerFunc(rh.GenerateApplicationPackage))))
	mux.Handle("/api/generate-pdf", requireAuth(http.HandlerFunc(rh.GeneratePDF)))
	mux.Handle("/api/builder/generate-pdf", requireAuth(http.HandlerFunc(bh.GeneratePDF)))
	mux.HandleFunc("/api/waitlist/notify-signup", wh.NotifySignupHandler)
	mux.Handle("/api/waitlist/status", requireAuth(http.HandlerFunc(wh.StatusHandler)))
	mux.Handle("/api/admin/waitlist", requireAuth(http.HandlerFunc(wh.AdminListHandler)))
	mux.Handle("/api/admin/waitlist/update", requireAuth(http.HandlerFunc(wh.AdminUpdateHandler)))

	handler := httputil.WithCORS(mux)
	addr := config.ServerAddr()
	log.Printf("Server running on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
