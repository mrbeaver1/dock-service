package validator

import (
	"encoding/json"

	"github.com/mrbeaver1/dock-service/internal/api/dto"
)

type CreateDocumentRequestValidator interface {
	Validate(dto dto.CreateDocumentRequest) error
}

type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	return e.Field + ": " + e.Message
}

type createDocumentRequestValidator struct {
}

func NewCreateDocumentRequestValidator() CreateDocumentRequestValidator {
	return &createDocumentRequestValidator{}
}

func (c *createDocumentRequestValidator) Validate(dto dto.CreateDocumentRequest) error {
	if dto.Meta == nil {
		return &FieldError{
			Field:   "meta",
			Message: "is required",
		}
	}
	if dto.JSON != nil && !json.Valid(dto.JSON) {
		return &FieldError{
			Field:   "json",
			Message: "must contain valid JSON",
		}
	}
	return nil
}
