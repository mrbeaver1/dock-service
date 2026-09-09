package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
)

type Error struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

type Body struct {
	Error    *Error `json:"error,omitempty"`
	Response any    `json:"response,omitempty"`
	Data     any    `json:"data,omitempty"`
}

func Write(w http.ResponseWriter, r *http.Request, status int, body Body) {
	body.Response = omitNil(body.Response)
	body.Data = omitNil(body.Data)

	payload, err := json.Marshal(body)
	if err != nil {
		slog.ErrorContext(r.Context(), "encode response", "error", err)
		status = http.StatusInternalServerError
		payload = []byte(`{"error":{"code":500,"text":"internal server error"}}`)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	w.WriteHeader(status)

	if r.Method == http.MethodHead {
		return
	}

	if _, err := w.Write(payload); err != nil {
		slog.ErrorContext(r.Context(), "write response", "error", err)
	}
}

func Success(w http.ResponseWriter, r *http.Request, confirmation, data any) {
	Write(w, r, http.StatusOK, Body{Response: confirmation, Data: data})
}

func Fail(w http.ResponseWriter, r *http.Request, status, code int, text string) {
	Write(w, r, status, Body{Error: &Error{Code: code, Text: text}})
}

func omitNil(value any) any {
	if value == nil {
		return nil
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if v.IsNil() {
			return nil
		}
	}

	return value
}
