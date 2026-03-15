package resume

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	resumeCollection = "resumes"
	ttlDuration      = 7 * 24 * time.Hour
)

type resumeDocument struct {
	TexContent string    `firestore:"tex_content"`
	UploadedAt time.Time `firestore:"uploaded_at"`
	ExpireAt   time.Time `firestore:"expire_at"`
	FileSize   int       `firestore:"file_size"`
}

type Storage struct {
	client *firestore.Client
}

func NewStorage(client *firestore.Client) *Storage {
	return &Storage{client: client}
}

func UserStorageDirName(userID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID)))
	return fmt.Sprintf("user_%x", sum)
}

func (st *Storage) LoadPersistedBaseResume(ctx context.Context, state *State, userID string) error {
	doc, err := st.client.Collection(resumeCollection).Doc(userID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil
		}
		return fmt.Errorf("firestore read failed: %w", err)
	}

	var resume resumeDocument
	if err := doc.DataTo(&resume); err != nil {
		return fmt.Errorf("firestore unmarshal failed: %w", err)
	}

	if resume.TexContent == "" {
		return nil
	}

	// Defense-in-depth: Firestore TTL deletion is eventually consistent,
	// so check expiry in application code as well.
	if time.Now().After(resume.ExpireAt) {
		return nil
	}

	state.SetBaseResume(userID, resume.TexContent)
	log.Printf("Loaded persisted base resume for user %s (%d chars)", UserStorageDirName(userID), len(resume.TexContent))
	return nil
}

func (st *Storage) EnsureBaseResumeLoaded(ctx context.Context, state *State, userID string) error {
	if _, ok := state.GetBaseResume(userID); ok {
		return nil
	}
	return st.LoadPersistedBaseResume(ctx, state, userID)
}

func (st *Storage) PersistBaseResume(ctx context.Context, userID, content string) error {
	now := time.Now()
	doc := resumeDocument{
		TexContent: content,
		UploadedAt: now,
		ExpireAt:   now.Add(ttlDuration),
		FileSize:   len(content),
	}
	_, err := st.client.Collection(resumeCollection).Doc(userID).Set(ctx, doc)
	if err != nil {
		return fmt.Errorf("firestore write failed: %w", err)
	}
	return nil
}
