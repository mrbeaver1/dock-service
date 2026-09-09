package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/mrbeaver1/dock-service/internal/api/decoder"
	"github.com/mrbeaver1/dock-service/internal/api/dto"
	"github.com/mrbeaver1/dock-service/internal/api/middleware"
	"github.com/mrbeaver1/dock-service/internal/api/response"
	"github.com/mrbeaver1/dock-service/internal/models"
	"github.com/mrbeaver1/dock-service/internal/service"
)

type UserService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Authenticate(ctx context.Context, login, password string) (string, error)
}

type SessionService interface {
	Validate(context.Context, string) (models.Session, error)
	Revoke(context.Context, models.Session) error
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := h.sessionService.Revoke(r.Context(), middleware.SessionFromContext(r.Context())); err != nil {
		writeUserError(w, r, err)
		return
	}
	response.Success(w, r, map[string]bool{r.PathValue("token"): true}, nil)
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	credentials, err := decoder.CredentialsRequestToDto(r)
	if err != nil {
		writeUserError(w, r, err)
		return
	}
	login, err := h.userService.Register(r.Context(), credentials.Login, credentials.Password)
	if err != nil {
		writeUserError(w, r, err)
		return
	}
	response.Success(w, r, dto.RegisterResult{Login: login}, nil)
}

func (h *handler) authenticate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	credentials, err := decoder.CredentialsRequestToDto(r)
	if err != nil {
		writeUserError(w, r, err)
		return
	}
	token, err := h.userService.Authenticate(r.Context(), credentials.Login, credentials.Password)
	if err != nil {
		writeUserError(w, r, err)
		return
	}
	response.Success(w, r, dto.AuthResult{Token: token}, nil)
}

func writeUserError(w http.ResponseWriter, r *http.Request, err error) {
	if middleware.WriteBodyError(w, r, err) {
		return
	}
	if errors.Is(err, service.ErrBusy) {
		w.Header().Set("Retry-After", "1")
		response.Fail(w, r, 503, 503, "authentication capacity exhausted")
		return
	}
	var decodeErr *decoder.DecodeError
	if errors.As(err, &decodeErr) {
		response.Fail(w, r, 400, 400, decodeErr.Error())
		return
	}
	for _, invalid := range []error{service.ErrInvalidLogin, service.ErrInvalidPassword, service.ErrLoginTaken} {
		if errors.Is(err, invalid) {
			response.Fail(w, r, 400, 400, invalid.Error())
			return
		}
	}
	if errors.Is(err, service.ErrInvalidCredentials) {
		response.Fail(w, r, 401, 401, service.ErrInvalidCredentials.Error())
		return
	}
	slog.ErrorContext(r.Context(), "user request failed", "error", err)
	response.Fail(w, r, 500, 500, "internal server error")
}
