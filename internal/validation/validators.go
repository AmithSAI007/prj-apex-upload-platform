package validation

import (
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
)

type ValidationContext struct {
	AllowedContentTypes []string
	MaxFileSizeBytes    int64
}

func NewValidationContext(allowedContentTypes []string, maxFileSizeBytes int64) *ValidationContext {
	normalized := make([]string, len(allowedContentTypes))
	for i, ct := range allowedContentTypes {
		normalized[i] = strings.ToLower(strings.TrimSpace(ct))
	}
	return &ValidationContext{
		AllowedContentTypes: normalized,
		MaxFileSizeBytes:    maxFileSizeBytes,
	}
}

func (vc *ValidationContext) RegisterValidators(v *validator.Validate) error {
	if err := v.RegisterValidation("contenttype", vc.validateContentType); err != nil {
		return err
	}
	if err := v.RegisterValidation("maxfilesize", vc.validateFileSize); err != nil {
		return err
	}
	if err := v.RegisterValidation("safefilename", validateSafeFileName); err != nil {
		return err
	}
	return nil
}

func (vc *ValidationContext) validateContentType(fl validator.FieldLevel) bool {
	contentType := fl.Field().String()
	if contentType == "" {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	if len(vc.AllowedContentTypes) == 0 {
		return true
	}
	for _, allowed := range vc.AllowedContentTypes {
		if normalized == allowed {
			return true
		}
	}
	return false
}

func (vc *ValidationContext) validateFileSize(fl validator.FieldLevel) bool {
	sizeBytes := fl.Field().Int()
	if sizeBytes <= 0 {
		return false
	}
	if vc.MaxFileSizeBytes <= 0 {
		return true
	}
	return sizeBytes <= vc.MaxFileSizeBytes
}

func validateSafeFileName(fl validator.FieldLevel) bool {
	fileName := fl.Field().String()
	if fileName == "" {
		return false
	}
	clean := filepath.Clean(fileName)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/") || strings.Contains(clean, "\\") {
		return false
	}
	return true
}
