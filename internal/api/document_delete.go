package api

import (
	"net/http"

	"github.com/mrbeaver1/dock-service/internal/api/decoder"
	"github.com/mrbeaver1/dock-service/internal/api/middleware"
	"github.com/mrbeaver1/dock-service/internal/api/response"
)

func (h *handler) deleteDocument(w http.ResponseWriter, r *http.Request) {
	id, err := decoder.DocumentID(r)
	if err != nil {
		writeDocumentError(w, r, err)
		return
	}
	if err := h.documentService.DeleteDocument(r.Context(), id, middleware.UserIDFromContext(r.Context())); err != nil {
		writeDocumentError(w, r, err)
		return
	}
	response.Success(w, r, map[string]bool{r.PathValue("id"): true}, nil)
}
