package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/mrbeaver1/dock-service/internal/api/decoder"
	"github.com/mrbeaver1/dock-service/internal/api/dto"
	"github.com/mrbeaver1/dock-service/internal/api/middleware"
	"github.com/mrbeaver1/dock-service/internal/api/response"
	"github.com/mrbeaver1/dock-service/internal/api/validator"
)

type Handler interface {
	Routes() http.Handler
}

type DocumentService interface {
	UploadDocument(ctx context.Context, request dto.CreateDocumentRequest, ownerId string) (dto.CreateDocumentResult, error)
	GetDocumentsList(ctx context.Context, request dto.GetDocumentsRequest, ownerId string) (dto.GetDocumentsResult, error)
}

type UploadDocumentRequestValidator interface {
	Validate(dto dto.CreateDocumentRequest) error
}

type handler struct {
	documentService                DocumentService
	uploadDocumentRequestValidator UploadDocumentRequestValidator
	rateLimiter                    *middleware.IPLimiter
}

func (h *handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /api/docs",
		h.getDocuments,
	)

	mux.HandleFunc("POST /api/docs", h.uploadDocument)

	mux.HandleFunc(
		"GET /api/docs/{id}",
		h.getDocument,
	)

	mux.HandleFunc("/api/docs", methodNotAllowed)
	mux.HandleFunc("/api/docs/{id}", methodNotAllowed)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response.Fail(w, r, http.StatusNotFound, http.StatusNotFound, http.StatusText(http.StatusNotFound))
	})

	return middleware.Recovery(
		middleware.RequestID(
			middleware.Logger(
				middleware.RateLimit(h.rateLimiter)(mux),
			),
		),
	)
}

func (h *handler) getDocuments(w http.ResponseWriter, r *http.Request) {
	reqDto, err := decoder.GetListRequestToDto(r)

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	ownerId := middleware.UserIDFromContext(r.Context())

	resp, err := h.documentService.GetDocumentsList(r.Context(), reqDto, ownerId.(string))

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	response.Success(w, r, nil, resp)
}

func (h *handler) uploadDocument(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r.MultipartForm != nil {
			if err := r.MultipartForm.RemoveAll(); err != nil {
				slog.ErrorContext(r.Context(), "remove multipart files", "error", err)
			}
		}
	}()

	err := r.ParseMultipartForm(0)

	if err != nil {
		slog.ErrorContext(r.Context(), "parse multipart form", "error", err)
		writeMultipartError(w, r, err)
		return
	}

	reqDto, closeFile, err := decoder.UploadRequestToDto(r)

	defer func() {
		err := closeFile()

		if err != nil {
			slog.ErrorContext(r.Context(), "close file", "error", err)
		}
	}()

	if err != nil {
		slog.ErrorContext(r.Context(), "upload request", "error", err)
		writeDocumentError(w, r, err)
		return
	}

	if err := h.uploadDocumentRequestValidator.Validate(reqDto); err != nil {
		slog.ErrorContext(r.Context(), "upload request", "error", err)
		writeDocumentError(w, r, err)
		return
	}

	ownerId := middleware.UserIDFromContext(r.Context())

	resp, err := h.documentService.UploadDocument(r.Context(), reqDto, ownerId.(string))

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	response.Success(w, r, nil, resp)
}

func (h *handler) getDocument(w http.ResponseWriter, r *http.Request) {
	response.Fail(w, r, http.StatusNotImplemented, http.StatusNotImplemented, http.StatusText(http.StatusNotImplemented))
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET, HEAD")
	response.Fail(w, r, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
}

func writeDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	var decodeErr *decoder.DecodeError
	if errors.As(err, &decodeErr) {
		response.Fail(w, r, 400, 400, decodeErr.Field+": must be a JSON object with valid field types")
		return
	}
	var fieldErr *validator.FieldError
	if errors.As(err, &fieldErr) {
		response.Fail(w, r, 400, 400, fieldErr.Error())
		return
	}
	writeInternalError(w, r, err)
}

func writeMultipartError(w http.ResponseWriter, r *http.Request, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		writeInternalError(w, r, err)
		return
	}
	response.Fail(w, r, 400, 400, "invalid multipart request")
}

func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "create document failed", "error", err)
	response.Fail(w, r, 500, 500, "internal server error")
}
