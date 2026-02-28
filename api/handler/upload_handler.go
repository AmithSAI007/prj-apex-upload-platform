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

type UploadHandler struct {
	// Add any dependencies here, e.g., services, repositories, etc.
	logger    *zap.Logger
	validator *validator.Validate
	service   service.UploadInterface
}

func NewUploadHandler(logger *zap.Logger, validator *validator.Validate, uploadService service.UploadInterface) *UploadHandler {
	return &UploadHandler{
		logger:    logger,
		validator: validator,
		service:   uploadService,
	}
}

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
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID))
		bindSpan.RecordError(err)
		bindSpan.SetStatus(codes.Error, "Failed to bind request body")
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Invalid request body",
				RequestID: traceID,
			},
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.logger.Error("Validation failed for request body", zap.Error(err), zap.String("trace_id", traceID))
		bindSpan.RecordError(err)
		bindSpan.SetStatus(codes.Error, "Validation failed for request body")
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

	// set useful attributes for downstream tracing and observability
	bindSpan.SetAttributes(
		attribute.String("file_name", req.FileName),
		attribute.String("trace_id", traceID),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxTenantIDKey))),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx.Request.Context(), string(constants.CtxUserIDKey))),
	)

	// propagate the span context into service calls by using the bind span context
	response, err := h.service.CreateUploadSession(bindCtx, req)
	if err != nil {
		if errors.Is(err, internalerrors.ErrInvalidInput) {
			h.logger.Warn("Invalid upload create request", zap.Error(err), zap.String("trace_id", traceID))
			bindSpan.RecordError(err)
			bindSpan.SetStatus(codes.Error, "Invalid upload create request")
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
	ctx.JSON(201, response)
}

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

	h.logger.Info("Resume upload session requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))
	response, err := h.service.ResumeUploadSession(svcCtx, uploadID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to resume upload session")
		h.respondWithServiceError(ctx, err, "Failed to resume upload session")
		return
	}

	span.SetStatus(codes.Ok, "Upload session resumed successfully")
	ctx.JSON(200, response)
}

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

	h.logger.Info("Get upload status requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))
	response, err := h.service.GetUploadStatus(svcCtx, uploadID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to fetch upload status")
		h.respondWithServiceError(ctx, err, "Failed to fetch upload status")
		return
	}

	span.SetStatus(codes.Ok, "Upload status fetched successfully")
	ctx.JSON(200, response)
}

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

	h.logger.Info("Query upload status requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))
	var req dto.QueryStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to bind request body")
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
		h.respondWithServiceError(ctx, err, "Failed to query upload status")
		return
	}

	span.SetStatus(codes.Ok, "Upload status queried successfully")
	ctx.JSON(200, response)
}

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

	h.logger.Info("Cancel upload session requested", zap.String("upload_id", uploadID), zap.String("trace_id", traceID))
	var req dto.CancelUploadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID))
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to bind request body")
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
		h.respondWithServiceError(ctx, err, "Failed to cancel upload session")
		return
	}

	span.SetStatus(codes.Ok, "Upload session cancelled successfully")
	ctx.JSON(200, response)
}

func (h *UploadHandler) respondWithServiceError(ctx *gin.Context, err error, message string) {
	traceID := pkgtrace.TraceIDFromContext(ctx.Request.Context())
	requestID := traceID
	// Attach error details to the current span if present
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
