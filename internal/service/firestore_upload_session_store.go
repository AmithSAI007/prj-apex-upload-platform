package service

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const uploadSessionsCollection = "upload_sessions"

// FirestoreUploadSessionStore stores upload sessions in Firestore.
type FirestoreUploadSessionStore struct {
	client *firestore.Client
}

// NewFirestoreUploadSessionStore creates a Firestore-backed session store.
func NewFirestoreUploadSessionStore(client *firestore.Client) *FirestoreUploadSessionStore {
	return &FirestoreUploadSessionStore{client: client}
}

func (s *FirestoreUploadSessionStore) Create(ctx context.Context, session *model.UploadSession) error {
	if session == nil {
		return errors.New("session is required")
	}

	_, err := s.client.Collection(uploadSessionsCollection).Doc(session.UploadID).Create(ctx, session)
	return err
}

func (s *FirestoreUploadSessionStore) GetByID(ctx context.Context, uploadID string) (*model.UploadSession, error) {
	snap, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, internalerrors.ErrNotFound
		}
		return nil, err
	}

	var session model.UploadSession
	if err := snap.DataTo(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *FirestoreUploadSessionStore) GetByIdempotencyKey(ctx context.Context, tenantID string, userID string, idempotencyKey string) (*model.UploadSession, error) {
	if idempotencyKey == "" {
		return nil, nil
	}

	query := s.client.Collection(uploadSessionsCollection).
		Where("tenantId", "==", tenantID).
		Where("userId", "==", userID).
		Where("idempotencyKey", "==", idempotencyKey).
		Limit(1)

	iter := query.Documents(ctx)
	defer iter.Stop()

	snap, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var session model.UploadSession
	if err := snap.DataTo(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *FirestoreUploadSessionStore) UpdateStatus(ctx context.Context, uploadID string, status model.UploadStatus, uploadedBytes int64) error {
	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: status},
		{Path: "uploadedBytes", Value: uploadedBytes},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	return err
}

func (s *FirestoreUploadSessionStore) UpdateGCSUploadURL(ctx context.Context, uploadID string, gcsUploadURL string) error {
	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "gcsUploadUrl", Value: gcsUploadURL},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	return err
}

func (s *FirestoreUploadSessionStore) MarkCompleted(ctx context.Context, uploadID string, uploadedBytes int64) error {
	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: model.StatusCompleted},
		{Path: "uploadedBytes", Value: uploadedBytes},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	return err
}

func (s *FirestoreUploadSessionStore) MarkCancelled(ctx context.Context, uploadID string) error {
	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: model.StatusCancelled},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	return err
}

func (s *FirestoreUploadSessionStore) MarkExpired(ctx context.Context, uploadID string) error {
	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: model.StatusExpired},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	return err
}

var _ UploadSessionStore = (*FirestoreUploadSessionStore)(nil)
