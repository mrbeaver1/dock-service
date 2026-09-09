package api

import (
	"context"
	"errors"

	"io"
	"log/slog"
	"net/http"
	"strconv"

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
	UploadDocument(ctx context.Context, request dto.CreateDocumentRequest, ownerId string) (dto.CreateDocumentResult, error)
	GetDocumentsList(ctx context.Context, request dto.GetDocumentsRequest, ownerId string) (dto.GetDocumentsResult, error)
}

type DocumentReader interface {
	GetDocument(context.Context, uuid.UUID, uuid.UUID, bool) (service.DocumentContent, error)
}

type UploadDocumentRequestValidator interface {
	Validate(dto dto.CreateDocumentRequest) error
}

type handler struct {
	documentReader                 DocumentReader
	userService                    UserService
	authSecret                     []byte
	adminToken                     string
	documentService                DocumentService
	uploadDocumentRequestValidator UploadDocumentRequestValidator
	rateLimiter                    *middleware.IPLimiter
}

func NewHandler(documents DocumentReader, users UserService, limiter *middleware.IPLimiter, secret []byte, adminToken string) Handler {
	return &handler{documentReader: documents, userService: users, rateLimiter: limiter, authSecret: secret,
		adminToken:                     adminToken,
		uploadDocumentRequestValidator: validator.NewCreateDocumentRequestValidator()}
}

func (h *handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/register", middleware.AdminAuth(h.adminToken, decoder.FormToken)(http.HandlerFunc(h.register)))
	mux.HandleFunc("POST /api/auth", h.authenticate)
	mux.Handle("DELETE /api/auth/{token}", middleware.Auth(h.authSecret, decoder.PathToken)(http.HandlerFunc(notImplemented)))

	mux.Handle(
		"GET /api/docs",
		middleware.Auth(h.authSecret, decoder.QueryToken)(http.HandlerFunc(h.getDocuments)),
	)

	mux.Handle("POST /api/docs", middleware.Auth(h.authSecret, decoder.MultipartToken)(http.HandlerFunc(h.uploadDocument)))
	mux.Handle("DELETE /api/docs/{id}", middleware.Auth(h.authSecret, decoder.QueryToken)(http.HandlerFunc(notImplemented)))

	mux.Handle(
		"GET /api/docs/{id}",
		middleware.Auth(h.authSecret, decoder.QueryToken)(http.HandlerFunc(h.getDocument)),
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

	return middleware.Recovery(
		middleware.RequestID(
			middleware.Logger(
				middleware.RateLimit(h.rateLimiter)(mux),
			),
		),
	)
}

func (h *handler) getDocuments(w http.ResponseWriter, r *http.Request) {
	if h.documentService == nil {
		response.Fail(w, r, 501, 501, "not implemented")
		return
	}
	reqDto, err := decoder.GetListRequestToDto(r)

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	ownerId := middleware.UserIDFromContext(r.Context())

	resp, err := h.documentService.GetDocumentsList(r.Context(), reqDto, ownerId.String())

	if err != nil {
		writeDocumentError(w, r, err)
		return
	}

	response.Success(w, r, nil, resp)
}

func (h *handler) uploadDocument(w http.ResponseWriter, r *http.Request) {
	if h.documentService == nil {
		response.Fail(w, r, 501, 501, "not implemented")
		return
	}
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

	ownerId := middleware.UserIDFromContext(r.Context())

	resp, err := h.documentService.UploadDocument(r.Context(), reqDto, ownerId.String())

	if err != nil {
		writeDocumentError(w, r, err)
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
	result, err := h.documentReader.GetDocument(r.Context(), id, middleware.UserIDFromContext(r.Context()), r.Method == http.MethodHead)
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

func notImplemented(w http.ResponseWriter, r *http.Request) {
	response.Fail(w, r, 501, 501, "not implemented")
}

func writeDocumentError(w http.ResponseWriter, r *http.Request, err error) {
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
