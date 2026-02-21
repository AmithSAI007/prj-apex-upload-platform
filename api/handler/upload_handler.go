package handler

import (
	"errors"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/api/middleware"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
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
	// TODO: implement create upload session
	var req dto.CreateUploadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID(ctx)))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Invalid request body",
				RequestID: traceID(ctx),
			},
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.logger.Error("Validation failed for request body", zap.Error(err), zap.String("trace_id", traceID(ctx)))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Validation failed",
				RequestID: traceID(ctx),
				Details:   []dto.ErrorDetail{{Message: err.Error()}},
			},
		})
		return
	}

	response, err := h.service.CreateUploadSession(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, internalerrors.ErrInvalidInput) {
			h.logger.Warn("Invalid upload create request", zap.Error(err), zap.String("trace_id", traceID(ctx)))
			ctx.JSON(400, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeInvalidArgument,
					Message:   "Invalid request",
					RequestID: traceID(ctx),
				},
			})
			return
		}
		h.logger.Error("Failed to create upload session", zap.Error(err), zap.String("trace_id", traceID(ctx)))
		ctx.JSON(500, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInternal,
				Message:   "Failed to create upload session",
				RequestID: traceID(ctx),
			},
		})
		return
	}

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
	// TODO: implement resume upload session
	h.logger.Info("Resume upload session requested", zap.String("upload_id", ctx.Param("uploadId")), zap.String("trace_id", traceID(ctx)))
	response, err := h.service.ResumeUploadSession(ctx.Request.Context(), ctx.Param("uploadId"))
	if err != nil {
		h.respondWithServiceError(ctx, err, "Failed to resume upload session")
		return
	}
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
	// TODO: implement get upload session status
	h.logger.Info("Get upload status requested", zap.String("upload_id", ctx.Param("uploadId")), zap.String("trace_id", traceID(ctx)))
	response, err := h.service.GetUploadStatus(ctx.Request.Context(), ctx.Param("uploadId"))
	if err != nil {
		h.respondWithServiceError(ctx, err, "Failed to fetch upload status")
		return
	}
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
	// TODO: implement query GCS status
	h.logger.Info("Query upload status requested", zap.String("upload_id", ctx.Param("uploadId")), zap.String("trace_id", traceID(ctx)))
	var req dto.QueryStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID(ctx)))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Invalid request body",
				RequestID: traceID(ctx),
			},
		})
		return
	}

	response, err := h.service.QueryUploadStatus(ctx.Request.Context(), ctx.Param("uploadId"), req)
	if err != nil {
		h.respondWithServiceError(ctx, err, "Failed to query upload status")
		return
	}
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
	// TODO: implement cancel upload session
	h.logger.Info("Cancel upload session requested", zap.String("upload_id", ctx.Param("uploadId")), zap.String("trace_id", traceID(ctx)))
	var req dto.CancelUploadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind request body", zap.Error(err), zap.String("trace_id", traceID(ctx)))
		ctx.JSON(400, dto.ErrorResponse{
			Error: dto.ErrorPayload{
				Code:      dto.ErrorCodeInvalidArgument,
				Message:   "Invalid request body",
				RequestID: traceID(ctx),
			},
		})
		return
	}

	response, err := h.service.CancelUploadSession(ctx.Request.Context(), ctx.Param("uploadId"), req)
	if err != nil {
		h.respondWithServiceError(ctx, err, "Failed to cancel upload session")
		return
	}
	ctx.JSON(200, response)
}

func (h *UploadHandler) respondWithServiceError(ctx *gin.Context, err error, message string) {
	requestID := traceID(ctx)
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

func traceID(ctx *gin.Context) string {
	if value, ok := ctx.Get(middleware.TraceIDKey); ok {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}
