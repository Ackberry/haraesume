package waitlist

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/auth"
	"backend/email"
	"backend/httputil"
)

type Handler struct {
	store  *Store
	mailer *email.Sender
}

func NewHandler(store *Store, mailer *email.Sender) *Handler {
	return &Handler{store: store, mailer: mailer}
}

func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := auth.RequestUserID(r)
	userEmail := auth.UserEmailFromContext(r.Context())

	entry, err := h.store.RegisterOrGet(userID, userEmail)
	if err != nil {
		log.Printf("waitlist registerOrGet failed for %s: %v", userID, err)
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to check waitlist status")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     entry.Status,
		"created_at": entry.CreatedAt,
	})
}

type adminUpdateRequest struct {
	UserID string `json:"user_id"`
	Status Status `json:"status"`
}

func (h *Handler) AdminListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := auth.RequestUserID(r)
	if !auth.IsAdminUID(userID) {
		httputil.WriteError(w, http.StatusForbidden, "Admin access required")
		return
	}

	entries := h.store.ListAll()

	type waitlistUserResponse struct {
		UserID    string    `json:"user_id"`
		Email     string    `json:"email"`
		Status    Status    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	users := make([]waitlistUserResponse, 0, len(entries))
	for uid, entry := range entries {
		users = append(users, waitlistUserResponse{
			UserID:    uid,
			Email:     entry.Email,
			Status:    entry.Status,
			CreatedAt: entry.CreatedAt,
			UpdatedAt: entry.UpdatedAt,
		})
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"users": users,
	})
}

func (h *Handler) AdminUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := auth.RequestUserID(r)
	if !auth.IsAdminUID(userID) {
		httputil.WriteError(w, http.StatusForbidden, "Admin access required")
		return
	}

	var payload adminUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if payload.UserID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	switch payload.Status {
	case StatusPending, StatusInvited, StatusApproved:
	default:
		httputil.WriteError(w, http.StatusBadRequest, "status must be one of: pending, invited, approved")
		return
	}

	entry, exists := h.store.GetEntry(payload.UserID)
	if !exists {
		httputil.WriteError(w, http.StatusNotFound, fmt.Sprintf("user %s not found in waitlist", payload.UserID))
		return
	}

	if err := h.store.UpdateStatus(payload.UserID, payload.Status); err != nil {
		httputil.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	log.Printf("Admin %s updated user %s to %s", userID, payload.UserID, payload.Status)

	if payload.Status == StatusApproved && entry.Email != "" {
		if err := h.mailer.SendApprovalEmail(entry.Email); err != nil {
			log.Printf("Approval email failed for %s: %v", entry.Email, err)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("User status updated to %s", payload.Status),
	})
}

type notifySignupRequest struct {
	Email string `json:"email"`
}

func (h *Handler) NotifySignupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var payload notifySignupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	emailAddr := strings.TrimSpace(strings.ToLower(payload.Email))
	if emailAddr == "" || !strings.Contains(emailAddr, "@") {
		httputil.WriteError(w, http.StatusBadRequest, "Valid email is required")
		return
	}

	if err := h.mailer.SendWaitlistConfirmation(emailAddr); err != nil {
		log.Printf("waitlist confirmation email failed: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to send confirmation email")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Confirmation email sent",
	})
}
