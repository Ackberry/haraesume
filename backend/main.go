package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
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

	// ── Services that are fast to init (no network) ──────────────────────
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

	bh := builder.NewHandler()
	wh := waitlist.NewHandler(wlStore, mailer)
	requireAuth := auth.RequireAuth(authValidator)
	llmLimiter := httputil.NewRateLimiter(1*time.Minute, auth.RequestUserID)

	// ── Firestore + JWKS init in background so server starts listening fast ──
	var ready atomic.Bool
	var rh *resume.Handler

	go func() {
		// Pre-warm JWKS (non-blocking, best-effort).
		if authValidator != nil {
			jwksCtx, jwksCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := authValidator.WarmUp(jwksCtx); err != nil {
				log.Printf("JWKS pre-warm failed (will retry on first request): %v", err)
			} else {
				log.Println("JWKS keys pre-warmed")
			}
			jwksCancel()
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

		// Verify Firestore connectivity.
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
		iter := fsClient.Collection("resumes").Limit(1).Documents(pingCtx)
		_, _ = iter.Next()
		iter.Stop()
		pingCancel()
		log.Println("Firestore connectivity verified")

		resumeState := resume.NewState()
		resumeStorage := resume.NewStorage(fsClient)
		rh = resume.NewHandler(resumeState, resumeStorage)
		ready.Store(true)
		log.Println("Backend fully initialized")
	}()

	// Gate that returns 503 until Firestore-dependent handlers are ready.
	requireReady := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ready.Load() {
				httputil.WriteError(w, http.StatusServiceUnavailable, "server is starting up, please retry in a moment")
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// Lazy wrapper — rh is nil until the goroutine sets it.
	resumeHandler := func(fn func(*resume.Handler) http.HandlerFunc) http.Handler {
		return requireReady(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fn(rh).ServeHTTP(w, r)
		}))
	}

	mux := http.NewServeMux()

	// Health check — always available, no deps.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		status := "healthy"
		if !ready.Load() {
			status = "starting"
		}
		httputil.WriteJSON(w, http.StatusOK, httputil.HealthResponse{
			Status:  status,
			Version: config.AppVersion,
		})
	})

	// Routes that need Firestore (gated by requireReady).
	mux.Handle("/api/resume-status", requireAuth(resumeHandler(func(h *resume.Handler) http.HandlerFunc { return h.ResumeStatus })))
	mux.Handle("/api/upload-resume", requireAuth(resumeHandler(func(h *resume.Handler) http.HandlerFunc { return h.UploadResume })))
	mux.Handle("/api/job-description", requireAuth(resumeHandler(func(h *resume.Handler) http.HandlerFunc { return h.SetJobDescription })))
	mux.Handle("/api/optimize",
		requireAuth(httputil.WithRateLimit(llmLimiter, resumeHandler(func(h *resume.Handler) http.HandlerFunc { return h.OptimizeResume }))))
	mux.Handle("/api/generate-application-package",
		requireAuth(httputil.WithRateLimit(llmLimiter, resumeHandler(func(h *resume.Handler) http.HandlerFunc { return h.GenerateApplicationPackage }))))
	mux.Handle("/api/generate-pdf", requireAuth(resumeHandler(func(h *resume.Handler) http.HandlerFunc { return h.GeneratePDF })))

	// Routes that DON'T need Firestore — available immediately.
	mux.Handle("/api/builder/generate-pdf", requireAuth(http.HandlerFunc(bh.GeneratePDF)))
	mux.HandleFunc("/api/waitlist/notify-signup", wh.NotifySignupHandler)
	mux.Handle("/api/waitlist/status", requireAuth(http.HandlerFunc(wh.StatusHandler)))
	mux.Handle("/api/admin/waitlist", requireAuth(http.HandlerFunc(wh.AdminListHandler)))
	mux.Handle("/api/admin/waitlist/update", requireAuth(http.HandlerFunc(wh.AdminUpdateHandler)))

	handler := httputil.WithCORS(mux)
	addr := config.ServerAddr()
	log.Printf("Server listening on %s (Firestore initializing in background)", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
