# Agents

This repository contains no autonomous agents. The primary service is a Go-based Cloud Run API that issues and manages Google Cloud Storage (GCS) resumable upload sessions for web clients.

## Product goal

Provide an enterprise-grade upload platform where authenticated web clients upload directly to GCS using resumable sessions created by this API. The service persists session state in Firestore and exposes REST endpoints for create, resume, status, and cancel. Server-side progress streaming is out of scope; clients track progress locally.

## Core requirements

- Scope: GCS only (no S3, no local uploads, no server-side file proxying).
- Auth: verify JWTs (issuer/audience/exp) and enforce tenant + user authorization.
- Persistence: Firestore is the source of truth for upload session state.
- Reliability: idempotent session creation, retry-safe APIs, and resumable uploads.

## API surface (high level)

- `POST /v1/uploads` create a resumable upload session and return the GCS upload URL.
- `POST /v1/uploads/{uploadId}/resume` return the existing GCS upload URL.
- `GET /v1/uploads/{uploadId}` return server-side session status.
- `POST /v1/uploads/{uploadId}/status` query current uploaded bytes from GCS.
- `POST /v1/uploads/{uploadId}/cancel` cancel and delete the resumable session/object.

## Data model (Firestore)

Collection: `upload_sessions`
Key fields: uploadId, tenantId, userId, bucket, objectName, contentType, sizeBytes, status, gcsUploadUrl, uploadedBytes, idempotencyKey, createdAt, updatedAt, expiresAt.

## Deployment assumptions

- Cloud Run service with a dedicated service account.
- IAM roles: Storage Object Creator/Admin (as needed), Firestore access.
- Env vars: project, bucket, JWT issuer/audience/JWK URL, upload TTL.
