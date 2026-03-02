// Package handler implements the HTTP endpoint handlers for the upload
// session API. Each handler creates OpenTelemetry spans, validates requests,
// delegates to the service layer, and maps errors to HTTP status codes.
package handler

import (
	"errors"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"
	pkgtrace "github.com/AmithSAI007/prj-apex-upload-platform/pkg/trace"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UploadHandler handles all HTTP endpoints for upload session management.
// It validates incoming requests, delegates business logic to the service layer,
// and formats responses. Each method creates an OpenTelemetry span for tracing.
type UploadHandler struct {
	logger    *zap.Logger
	validator *validator.Validate
	service   service.UploadInterface
}

// NewUploadHandler constructs an UploadHandler with the given logger, request
// validator, and upload service implementation.
func NewUploadHandler(logger *zap.Logger, validator *validator.Validate, uploadService service.UploadInterface) *UploadHandler {
	return &UploadHandler{
		logger:    logger,
		validator: validator,
		service:   uploadService,
	}
}

// Create handles POST /v1/uploads.
// It binds and validates the JSON request body, calls the service layer to
// create a new resumable upload session, and returns the GCS upload URL.
//
// @Summary Create upload session
// @Description Create a resumable upload session and return a signed GCS upload URL.
// @Tags uploads
// @Accept json
// @Produce json
// @Param X-Request-ID header string false "Correlation ID"
// @Param X-Tenant-ID header string false "Tenant ID"
// @Param request body dto.CreateUploadRequest true "Create upload session"
// @Success 201 {object} dto.CreateUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /uploads [post]
func (h *UploadHandler) Create(ctx *gin.Context) {
	var req dto.CreateUploadRequest
	traceID := pkgtrace.TraceIDFromContext(ctx.Request.Context())
	trace := otel.Tracer("github.com/AmithSAI007/prj-apex-upload-platform")
	bindCtx, bindSpan := trace.Start(ctx.Request.Context(), "/api/handler/UploadHandler/BindCreateUploadRequest")
	defer bindSpan.End()

	// Bind the JSON request body into the DTO struct.
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID))
		bindSpan.RecordError(err)
		bindSpan.SetStatus(codes.Error, "Failed to bind request body")
		bindSpan.AddEvent("handler.bind.failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Invalid request body",
				RequestID: traceID,
			},
		})
		return
	}

	// Run struct-level validation (required fields, value constraints).
	if err := h.validator.Struct(req); err != nil {
		h.logger.Error("Validation failed for request body", zap.Error(err), zap.String("trace_id", traceID))
		bindSpan.RecordError(err)
		bindSpan.SetStatus(codes.Error, "Validation failed for request body")
		bindSpan.AddEvent("handler.validation.failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Validation failed",
				RequestID: traceID,
				Details:   []dto.ErrorDetail{{Message: err.Error()}},
			},
		})
		return
	}

	// Set useful attributes for downstream tracing and observability.
	bindSpan.SetAttributes(
		attribute.String("file_name", req.FileName),
		attribute.String("trace_id", traceID),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxTenantIDKey))),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxUserIDKey))),
	)
	bindSpan.AddEvent("handler.create.request_validated")

	// Delegate to the service layer; propagate the span context into service calls.
	response, err := h.service.CreateUploadSession(bindCtx, req)
	if err != nil {
		if errors.Is(err, internalerrors.ErrInvalidInput) {
			h.logger.Warn("Invalid upload create request", zap.Error(err), zap.String("trace_id", traceID))
			bindSpan.RecordError(err)
			bindSpan.SetStatus(codes.Error, "Invalid upload create request")
			bindSpan.AddEvent("handler.create.invalid_input")
			ctx.JSON(400, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeInvalidArgument,
					Message:   "Invalid request",
					RequestID: traceID,
				},
			})
			return
		}
		h.logger.Error("Failed to create upload session", zap.Error(err), zap.String("trace_id", traceID))
		bindSpan.RecordError(err)
		bindSpan.SetStatus(codes.Error, "Failed to create upload session")
		bindSpan.AddEvent("handler.create.service_error", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		ctx.JSON(500, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInternal,
				Message:   "Failed to create upload session",
				RequestID: traceID,
			},
		})
		return
	}

	bindSpan.SetStatus(codes.Ok, "Upload session created successfully")
	bindSpan.AddEvent("handler.create.success", otrace.WithAttributes(
		attribute.String("upload_id", response.UploadID),
	))
	ctx.JSON(201, response)
}

// Resume handles POST /v1/uploads/{uploadId}/resume.
// It retrieves the existing upload session and returns the GCS upload URL
// so the client can continue an interrupted upload.
//
// @Summary Resume upload session
// @Description Return the existing resumable upload URL for a session.
// @Tags uploads
// @Accept json
// @Produce json
// @Param X-Request-ID header string false "Correlation ID"
// @Param X-Tenant-ID header string false "Tenant ID"
// @Param uploadId path string true "Upload ID"
// @Param request body dto.ResumeUploadRequest false "Resume upload session"
// @Success 200 {object} dto.ResumeUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 410 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /uploads/{uploadId}/resume [post]
func (h *UploadHandler) Resume(ctx *gin.Context) {
	traceID := pkgtrace.TraceIDFromContext(ctx.Request.Context())
	tracer := otel.Tracer("github.com/AmithSAI007/prj-apex-upload-platform")
	svcCtx, span := tracer.Start(ctx.Request.Context(), "/api/handler/UploadHandler/ResumeUploadSession")
	defer span.End()

	uploadID := ctx.Param("uploadId")
	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("trace_id", traceID),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxTenantIDKey))),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxUserIDKey))),
	)
	span.AddEvent("handler.resume.start")

	h.logger.Info("Resume upload session requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))
	response, err := h.service.ResumeUploadSession(svcCtx, uploadID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to resume upload session")
		span.AddEvent("handler.resume.failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		h.respondWithServiceError(ctx, err, "Failed to resume upload session")
		return
	}

	span.SetStatus(codes.Ok, "Upload session resumed successfully")
	span.AddEvent("handler.resume.success")
	ctx.JSON(200, response)
}

// GetStatus handles GET /v1/uploads/{uploadId}.
// It returns the server-side state of the upload session from Firestore.
//
// @Summary Get upload status
// @Description Get server-side status for an upload session.
// @Tags uploads
// @Produce json
// @Param X-Request-ID header string false "Correlation ID"
// @Param X-Tenant-ID header string false "Tenant ID"
// @Param uploadId path string true "Upload ID"
// @Success 200 {object} dto.UploadStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 410 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /uploads/{uploadId} [get]
func (h *UploadHandler) GetStatus(ctx *gin.Context) {
	traceID := pkgtrace.TraceIDFromContext(ctx.Request.Context())
	tracer := otel.Tracer("github.com/AmithSAI007/prj-apex-upload-platform")
	svcCtx, span := tracer.Start(ctx.Request.Context(), "/api/handler/UploadHandler/GetUploadStatus")
	defer span.End()

	uploadID := ctx.Param("uploadId")
	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("trace_id", traceID),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxTenantIDKey))),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxUserIDKey))),
	)
	span.AddEvent("handler.get_status.start")

	h.logger.Info("Get upload status requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))
	response, err := h.service.GetUploadStatus(svcCtx, uploadID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch upload status")
		span.AddEvent("handler.get_status.failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		h.respondWithServiceError(ctx, err, "Failed to fetch upload status")
		return
	}

	span.SetStatus(codes.Ok, "Upload status fetched successfully")
	span.AddEvent("handler.get_status.success")
	ctx.JSON(200, response)
}

// QueryStatus handles POST /v1/uploads/{uploadId}/status.
// It optionally queries GCS for the current uploaded byte count and updates
// the session state in Firestore.
//
// @Summary Query GCS upload status
// @Description Query current uploaded bytes from GCS for the session.
// @Tags uploads
// @Accept json
// @Produce json
// @Param X-Request-ID header string false "Correlation ID"
// @Param X-Tenant-ID header string false "Tenant ID"
// @Param uploadId path string true "Upload ID"
// @Param request body dto.QueryStatusRequest false "Query upload status"
// @Success 200 {object} dto.QueryStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 410 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /uploads/{uploadId}/status [post]
func (h *UploadHandler) QueryStatus(ctx *gin.Context) {
	traceID := pkgtrace.TraceIDFromContext(ctx.Request.Context())
	tracer := otel.Tracer("github.com/AmithSAI007/prj-apex-upload-platform")
	svcCtx, span := tracer.Start(ctx.Request.Context(), "/api/handler/UploadHandler/QueryUploadStatus")
	defer span.End()

	uploadID := ctx.Param("uploadId")
	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("trace_id", traceID),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxTenantIDKey))),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxUserIDKey))),
	)
	span.AddEvent("handler.query_status.start")

	h.logger.Info("Query upload status requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))

	// Bind the optional request body to determine whether to refresh from GCS.
	var req dto.QueryStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to bind request body")
		span.AddEvent("handler.query_status.bind_failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Invalid request body",
				RequestID: traceID,
			},
		})
		return
	}

	response, err := h.service.QueryUploadStatus(svcCtx, uploadID, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to query upload status")
		span.AddEvent("handler.query_status.failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		h.respondWithServiceError(ctx, err, "Failed to query upload status")
		return
	}

	span.SetStatus(codes.Ok, "Upload status queried successfully")
	span.AddEvent("handler.query_status.success")
	ctx.JSON(200, response)
}

// Cancel handles POST /v1/uploads/{uploadId}/cancel.
// It marks the upload session as cancelled in Firestore.
//
// @Summary Cancel upload session
// @Description Cancel the upload session and delete any partial object.
// @Tags uploads
// @Accept json
// @Produce json
// @Param X-Request-ID header string false "Correlation ID"
// @Param X-Tenant-ID header string false "Tenant ID"
// @Param uploadId path string true "Upload ID"
// @Param request body dto.CancelUploadRequest false "Cancel upload session"
// @Success 200 {object} dto.CancelUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 410 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /uploads/{uploadId}/cancel [post]
func (h *UploadHandler) Cancel(ctx *gin.Context) {
	traceID := pkgtrace.TraceIDFromContext(ctx.Request.Context())
	tracer := otel.Tracer("github.com/AmithSAI007/prj-apex-upload-platform")
	svcCtx, span := tracer.Start(ctx.Request.Context(), "/api/handler/UploadHandler/CancelUploadSession")
	defer span.End()

	uploadID := ctx.Param("uploadId")
	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("trace_id", traceID),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxTenantIDKey))),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxUserIDKey))),
	)
	span.AddEvent("handler.cancel.start")

	h.logger.Info("Cancel upload session requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))

	// Bind the optional request body for the cancellation reason.
	var req dto.CancelUploadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to bind request body")
		span.AddEvent("handler.cancel.bind_failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Invalid request body",
				RequestID: traceID,
			},
		})
		return
	}

	response, err := h.service.CancelUploadSession(svcCtx, uploadID, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to cancel upload session")
		span.AddEvent("handler.cancel.failed", otrace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		h.respondWithServiceError(ctx, err, "Failed to cancel upload session")
		return
	}

	span.SetStatus(codes.Ok, "Upload session cancelled successfully")
	span.AddEvent("handler.cancel.success")
	ctx.JSON(200, response)
}

// respondWithServiceError maps service-layer sentinel errors to the appropriate
// HTTP status code and error response. Uses errors.Is to classify the error:
//   - ErrInvalidInput -> 400 Bad Request
//   - ErrNotFound     -> 404 Not Found
//   - ErrSessionExpired -> 410 Gone
//   - anything else   -> 500 Internal Server Error
func (h *UploadHandler) respondWithServiceError(ctx *gin.Context, err error, message string) {
	traceID := pkgtrace.TraceIDFromContext(ctx.Request.Context())
	requestID := traceID

	// Attach error details to the current span if present.
	span := otrace.SpanFromContext(ctx.Request.Context())
	if span != nil && span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, message)
	}
	if errors.Is(err, internalerrors.ErrInvalidInput) {
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{Code: dto.ErrorCodeInvalidArgument, Message: message, RequestID: requestID},
		})
		return
	}
	if errors.Is(err, internalerrors.ErrNotFound) {
		ctx.JSON(404, dto.ErrorResponse{
			Error: dto.ErrorPayload{Code: dto.ErrorCodeNotFound, Message: "Upload session not found", RequestID: requestID},
		})
		return
	}
	if errors.Is(err, internalerrors.ErrSessionExpired) {
		ctx.JSON(410, dto.ErrorResponse{
			Error: dto.ErrorPayload{Code: dto.ErrorCodeGone, Message: "Upload session expired", RequestID: requestID},
		})
		return
	}
	ctx.JSON(500, dto.ErrorResponse{
		Error: dto.ErrorPayload{Code: dto.ErrorCodeInternal, Message: message, RequestID: requestID},
	})
}
