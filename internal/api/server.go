package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/api/decoder"
	"github.com/mrbeaver1/dock-service/internal/api/dto"
	"github.com/mrbeaver1/dock-service/internal/api/middleware"
	"github.com/mrbeaver1/dock-service/internal/api/response"
	"github.com/mrbeaver1/dock-service/internal/api/validator"
	"github.com/mrbeaver1/dock-service/internal/models"
	"github.com/mrbeaver1/dock-service/internal/service"
)

type Handler interface {
	Routes() http.Handler
}

type DocumentService interface {
	GetDocument(context.Context, uuid.UUID, uuid.UUID, bool) (service.DocumentContent, error)
	UploadDocument(context.Context, models.Document, []string, io.Reader) (models.Document, error)
	GetDocumentsList(context.Context, uuid.UUID, models.DocumentListOptions) ([]models.DocumentListItem, error)
	DeleteDocument(context.Context, uuid.UUID, uuid.UUID) error
}

type UploadDocumentRequestValidator interface {
	Validate(dto dto.CreateDocumentRequest) error
}

type handler struct {
	userService                    UserService
	sessionService                 SessionService
	adminToken                     string
	documentService                DocumentService
	uploadDocumentRequestValidator UploadDocumentRequestValidator
	rateLimiter                    *middleware.IPLimiter
	uploadLimiter                  *middleware.BodyLimiter
	credentialLimiter              *middleware.BodyLimiter
}

func NewHandler(documents DocumentService, users UserService, sessions SessionService, limiter *middleware.IPLimiter, uploads, credentials *middleware.BodyLimiter, adminToken string) Handler {
	return &handler{documentService: documents, userService: users, sessionService: sessions, rateLimiter: limiter,
		uploadLimiter:                  uploads,
		credentialLimiter:              credentials,
		adminToken:                     adminToken,
		uploadDocumentRequestValidator: validator.NewCreateDocumentRequestValidator()}
}

func (h *handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/register", h.credentialLimiter.Wrap(middleware.AdminAuth(h.adminToken, decoder.FormToken)(http.HandlerFunc(h.register))))
	mux.Handle("POST /api/auth", h.credentialLimiter.Wrap(http.HandlerFunc(h.authenticate)))
	mux.Handle("DELETE /api/auth/{token}", middleware.Auth(h.sessionService, decoder.PathToken)(http.HandlerFunc(h.logout)))

	mux.Handle(
		"GET /api/docs",
		middleware.Auth(h.sessionService, decoder.QueryToken)(http.HandlerFunc(h.getDocuments)),
	)

	mux.Handle("POST /api/docs", h.uploadLimiter.Wrap(middleware.Auth(h.sessionService, decoder.MultipartToken)(http.HandlerFunc(h.uploadDocument))))
	mux.Handle("DELETE /api/docs/{id}", middleware.Auth(h.sessionService, decoder.QueryToken)(http.HandlerFunc(h.deleteDocument)))

	mux.Handle(
		"GET /api/docs/{id}",
		middleware.Auth(h.sessionService, decoder.QueryToken)(http.HandlerFunc(h.getDocument)),
	)

	mux.HandleFunc("/api/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "GET, HEAD, POST")
		response.Fail(w, r, 405, 405, "method not allowed")
	})
	mux.HandleFunc("/api/docs/{id}", methodNotAllowed("GET, HEAD, DELETE"))
	mux.HandleFunc("/api/register", methodNotAllowed("POST"))
	mux.HandleFunc("/api/auth", methodNotAllowed("POST"))
	mux.HandleFunc("/api/auth/{token}", methodNotAllowed("DELETE"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response.Fail(w, r, http.StatusNotFound, http.StatusNotFound, http.StatusText(http.StatusNotFound))
	})

	return middleware.CloseRequestBody(middleware.Recovery(
		middleware.RequestID(
			middleware.Logger(
				middleware.RateLimit(h.rateLimiter)(mux),
			),
		),
	))
}

func (h *handler) getDocuments(w http.ResponseWriter, r *http.Request) {
	reqDto, err := decoder.GetListRequestToDto(r)

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	documents, err := h.documentService.GetDocumentsList(r.Context(), middleware.UserIDFromContext(r.Context()), reqDto)

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	resp := dto.GetDocumentsResult{Docs: make([]dto.DocumentListItem, 0, len(documents))}
	for _, document := range documents {
		resp.Docs = append(resp.Docs, dto.DocumentListItem{
			ID: document.ID, Name: document.Name, MIME: document.MIME, File: document.File,
			Public: document.Public, Created: document.CreatedAt.UTC().Format(time.DateTime), Grant: document.Grant,
		})
	}
	response.Success(w, r, nil, resp)
}

func (h *handler) uploadDocument(w http.ResponseWriter, r *http.Request) {
	reqDto, closeFile, err := decoder.UploadRequestToDto(r)

	if closeFile != nil {
		defer func() {
			if err := closeFile(); err != nil {
				slog.ErrorContext(r.Context(), "close file", "error", err)
			}
		}()
	}

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

	document := models.Document{
		OwnerID: middleware.UserIDFromContext(r.Context()), Name: reqDto.Meta.Name,
		File: reqDto.Meta.File, Public: reqDto.Meta.Public, MIME: reqDto.Meta.MIME, JSON: reqDto.JSON,
	}
	var content io.Reader
	if reqDto.File != nil {
		document.FileSize = &reqDto.File.Size
		content = reqDto.File.Content
	}
	saved, err := h.documentService.UploadDocument(r.Context(), document, reqDto.Meta.Grant, content)

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	resp := dto.CreateDocumentResult{JSON: saved.JSON}
	if reqDto.File != nil {
		resp.File = &reqDto.File.Filename
	}
	if resp.JSON == nil && resp.File == nil {
		response.Success(w, r, nil, nil)
		return
	}
	response.Success(w, r, nil, resp)
}

func (h *handler) getDocument(w http.ResponseWriter, r *http.Request) {
	id, err := decoder.DocumentID(r)
	if err != nil {
		writeDocumentError(w, r, err)
		return
	}
	result, err := h.documentService.GetDocument(r.Context(), id, middleware.UserIDFromContext(r.Context()), r.Method == http.MethodHead)
	if err != nil {
		writeDocumentError(w, r, err)
		return
	}
	if result.File == nil {
		response.Success(w, r, nil, result.JSON)
		return
	}
	file := result.File
	if file.Content != nil {
		defer func() {
			if err := file.Content.Close(); err != nil {
				slog.ErrorContext(r.Context(), "close document stream", "document_id", id, "error", err)
			}
		}()
	}
	if file.Size < 0 || (r.Method != http.MethodHead && file.Content == nil) {
		writeInternalError(w, r, errors.New("invalid file result"))
		return
	}
	w.Header().Set("Content-Type", file.MIME)
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	n, err := io.Copy(w, file.Content)
	if err != nil || n != file.Size {
		slog.ErrorContext(r.Context(), "stream document", "document_id", id, "bytes", n, "expected", file.Size, "error", err)
		panic(http.ErrAbortHandler)
	}
}

func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		response.Fail(w, r, 405, 405, "method not allowed")
	}
}

func writeDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, models.ErrUploadExpired) {
		w.Header().Set("Retry-After", "1")
		response.Fail(w, r, 503, 503, "upload expired; retry the request")
		return
	}
	if errors.Is(err, service.ErrInvalidDocument) || errors.Is(err, service.ErrInvalidListOptions) {
		response.Fail(w, r, 400, 400, err.Error())
		return
	}
	if errors.Is(err, models.ErrInvalidData) {
		response.Fail(w, r, 400, 400, "invalid document data")
		return
	}
	if errors.Is(err, models.ErrUserNotFound) {
		response.Fail(w, r, 404, 404, "user not found")
		return
	}
	if errors.Is(err, models.ErrDocumentNotFound) {
		response.Fail(w, r, 404, 404, "document not found")
		return
	}
	if errors.Is(err, service.ErrForbidden) {
		response.Fail(w, r, 403, 403, "document access denied")
		return
	}
	var decodeErr *decoder.DecodeError
	if errors.As(err, &decodeErr) {
		response.Fail(w, r, 400, 400, decodeErr.Error())
		return
	}
	var fieldErr *validator.FieldError
	if errors.As(err, &fieldErr) {
		response.Fail(w, r, 400, 400, fieldErr.Error())
		return
	}
	writeInternalError(w, r, err)
}

func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "document request failed", "error", err)
	response.Fail(w, r, 500, 500, "internal server error")
}
