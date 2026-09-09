package decoder

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"unicode/utf8"

	"github.com/mrbeaver1/dock-service/internal/api/dto"
)

func formValues(r *http.Request) (url.Values, error) {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/x-www-form-urlencoded" {
		return nil, &DecodeError{Field: "form", Err: errors.New("expected application/x-www-form-urlencoded")}
	}
	if err := r.ParseForm(); err != nil {
		return nil, &DecodeError{Field: "form", Err: err}
	}
	return r.PostForm, nil
}

func CredentialsRequestToDto(r *http.Request) (dto.Credentials, error) {
	form, err := formValues(r)
	if err != nil {
		return dto.Credentials{}, err
	}
	for _, field := range []string{"login", "pswd"} {
		values := form[field]
		if len(values) != 1 || values[0] == "" || !utf8.ValidString(values[0]) {
			return dto.Credentials{}, &DecodeError{Field: field, Err: errors.New("expected one non-empty UTF-8 value")}
		}
	}
	return dto.Credentials{Login: form.Get("login"), Password: form.Get("pswd")}, nil
}
