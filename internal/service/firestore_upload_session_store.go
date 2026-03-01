package service

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	otCodes "go.opentelemetry.io/otel/codes"
)

// uploadSessionsCollection is the Firestore collection name for upload session documents.
const uploadSessionsCollection = "upload_sessions"

// FirestoreUploadSessionStore stores upload sessions in Firestore.
type FirestoreUploadSessionStore struct {
	client *firestore.Client
	tracer trace.Tracer
	logger *zap.Logger
}

// NewFirestoreUploadSessionStore creates a Firestore-backed session store.
func NewFirestoreUploadSessionStore(client *firestore.Client, logger *zap.Logger, tracer trace.Tracer) *FirestoreUploadSessionStore {
	return &FirestoreUploadSessionStore{
		client: client,
		tracer: tracer,
		logger: logger,
	}
}

// Create persists a new upload session document in Firestore. The document ID
// is set to session.UploadID for deterministic lookups.
func (s *FirestoreUploadSessionStore) Create(ctx context.Context, session *model.UploadSession) error {
	if session == nil {
		return errors.New("session is required")
	}

	ctx, span := s.tracer.Start(ctx, "CreateUploadSession")
	span.SetAttributes(
		attribute.String("tenantId", session.TenantID),
		attribute.String("userId", session.UserID),
		attribute.String("uploadId", session.UploadID),
		attribute.String("idempotencyKey", session.IdempotencyKey),
		attribute.String("db.operation", "CREATE"),
		attribute.String("db.system", "firestore"),
	)
	defer span.End()

	span.AddEvent("firestore.create.start", trace.WithAttributes(
		attribute.String("collection", uploadSessionsCollection),
	))

	_, err := s.client.Collection(uploadSessionsCollection).Doc(session.UploadID).Create(ctx, session)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "Failed to create upload session in Firestore")
		span.AddEvent("firestore.create.error", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		return err
	}

	span.AddEvent("firestore.create.success", trace.WithAttributes(
		attribute.String("doc.id", session.UploadID),
	))
	span.SetStatus(otCodes.Ok, "Upload session created successfully")
	return err
}

// GetByID retrieves an upload session by its document ID (uploadID).
// Returns (nil, ErrNotFound) if the document does not exist in Firestore.
func (s *FirestoreUploadSessionStore) GetByID(ctx context.Context, uploadID string) (*model.UploadSession, error) {
	ctx, span := s.tracer.Start(ctx, "GetUploadSessionByID")
	span.SetAttributes(
		attribute.String("uploadId", uploadID),
		attribute.String("db.operation", "GET"),
		attribute.String("db.system", "firestore"),
	)
	defer span.End()

	snap, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			span.RecordError(err)
			span.SetStatus(otCodes.Error, "document not found")
			return nil, internalerrors.ErrNotFound
		}
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "failed to get document")
		return nil, err
	}

	var session model.UploadSession
	if err := snap.DataTo(&session); err != nil {
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "failed to parse document")
		return nil, err
	}

	span.SetStatus(otCodes.Ok, "fetched upload session")
	return &session, nil
}

// GetByIdempotencyKey queries for an existing session matching the given
// tenant, user, and idempotency key combination. Returns (nil, nil) if no
// matching session is found.
func (s *FirestoreUploadSessionStore) GetByIdempotencyKey(ctx context.Context, tenantID string, userID string, idempotencyKey string) (*model.UploadSession, error) {
	if idempotencyKey == "" {
		return nil, nil
	}

	idCtx, idSpan := s.tracer.Start(ctx, "GetByIdempotencyKey")
	idSpan.SetAttributes(
		attribute.String("tenantId", tenantID),
		attribute.String("userId", userID),
		attribute.String("idempotencyKey", idempotencyKey),
		attribute.String("db.operation", "GET"),
		attribute.String("db.system", "firestore"),
	)
	defer idSpan.End()

	idSpan.AddEvent("firestore.query.start", trace.WithAttributes(
		attribute.String("collection", uploadSessionsCollection),
	))

	query := s.client.Collection(uploadSessionsCollection).
		Where("tenantId", "==", tenantID).
		Where("userId", "==", userID).
		Where("idempotencyKey", "==", idempotencyKey).
		Limit(1)

	iter := query.Documents(idCtx)
	defer iter.Stop()

	if err := idCtx.Err(); err != nil {
		s.logger.Error("Context error before Firestore query", zap.Error(err))
	}

	snap, err := iter.Next()
	if err == iterator.Done {
		idSpan.AddEvent("firestore.query.no_result", trace.WithAttributes(
			attribute.String("collection", uploadSessionsCollection),
		))
		idSpan.SetStatus(otCodes.Ok, "No upload session found for idempotency key")
		return nil, nil
	}
	if err != nil {
		idSpan.RecordError(err)
		idSpan.SetStatus(otCodes.Error, "Firestore query failed")
		idSpan.AddEvent("firestore.query.error", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		return nil, err
	}

	idSpan.AddEvent("firestore.query.success", trace.WithAttributes(
		attribute.String("doc.id", snap.Ref.ID),
	))

	var session model.UploadSession
	if err := snap.DataTo(&session); err != nil {
		idSpan.RecordError(err)
		idSpan.SetStatus(otCodes.Error, "Failed to parse Firestore document")
		return nil, err
	}

	idSpan.SetStatus(otCodes.Ok, "Upload session found")
	return &session, nil
}

// UpdateStatus patches the session's status and uploaded byte count. Also
// bumps the updatedAt timestamp.
func (s *FirestoreUploadSessionStore) UpdateStatus(ctx context.Context, uploadID string, status model.UploadStatus, uploadedBytes int64) error {
	ctx, span := s.tracer.Start(ctx, "UpdateUploadStatus")
	span.SetAttributes(
		attribute.String("uploadId", uploadID),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.system", "firestore"),
	)
	defer span.End()

	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: status},
		{Path: "uploadedBytes", Value: uploadedBytes},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "failed to update upload status")
	} else {
		span.SetStatus(otCodes.Ok, "updated upload status")
	}
	return err
}

// UpdateGCSUploadURL replaces the stored GCS upload URL for the session.
func (s *FirestoreUploadSessionStore) UpdateGCSUploadURL(ctx context.Context, uploadID string, gcsUploadURL string) error {
	ctx, span := s.tracer.Start(ctx, "UpdateGCSUploadURL")
	span.SetAttributes(
		attribute.String("uploadId", uploadID),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.system", "firestore"),
	)
	defer span.End()

	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "gcsUploadUrl", Value: gcsUploadURL},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "failed to update gcs upload url")
	} else {
		span.SetStatus(otCodes.Ok, "updated gcs upload url")
	}
	return err
}

// MarkCompleted transitions the session to "completed" and records the final
// uploaded byte count.
func (s *FirestoreUploadSessionStore) MarkCompleted(ctx context.Context, uploadID string, uploadedBytes int64) error {
	ctx, span := s.tracer.Start(ctx, "MarkUploadCompleted")
	span.SetAttributes(
		attribute.String("uploadId", uploadID),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.system", "firestore"),
	)
	defer span.End()

	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: model.StatusCompleted},
		{Path: "uploadedBytes", Value: uploadedBytes},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "failed to mark completed")
	} else {
		span.SetStatus(otCodes.Ok, "marked completed")
	}
	return err
}

// MarkCancelled transitions the session to "cancelled".
func (s *FirestoreUploadSessionStore) MarkCancelled(ctx context.Context, uploadID string) error {
	ctx, span := s.tracer.Start(ctx, "MarkUploadCancelled")
	span.SetAttributes(
		attribute.String("uploadId", uploadID),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.system", "firestore"),
	)
	defer span.End()

	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: model.StatusCancelled},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "failed to mark cancelled")
	} else {
		span.SetStatus(otCodes.Ok, "marked cancelled")
	}
	return err
}

// MarkExpired transitions the session to "expired" when its TTL has elapsed.
func (s *FirestoreUploadSessionStore) MarkExpired(ctx context.Context, uploadID string) error {
	ctx, span := s.tracer.Start(ctx, "MarkUploadExpired")
	span.SetAttributes(
		attribute.String("uploadId", uploadID),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.system", "firestore"),
	)
	defer span.End()

	_, err := s.client.Collection(uploadSessionsCollection).Doc(uploadID).Update(ctx, []firestore.Update{
		{Path: "status", Value: model.StatusExpired},
		{Path: "updatedAt", Value: time.Now().UTC()},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otCodes.Error, "failed to mark expired")
	} else {
		span.SetStatus(otCodes.Ok, "marked expired")
	}
	return err
}

var _ UploadSessionStore = (*FirestoreUploadSessionStore)(nil)
