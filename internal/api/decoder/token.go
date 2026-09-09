package decoder

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

func QueryToken(r *http.Request) (string, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", &DecodeError{Field: "query", Err: err}
	}
	return singleToken(values["token"])
}

func PathToken(r *http.Request) (string, error) { return r.PathValue("token"), nil }

func FormToken(r *http.Request) (string, error) {
	form, err := formValues(r)
	if err != nil {
		return "", err
	}
	return singleToken(form["token"])
}

func MultipartToken(r *http.Request) (string, error) {
	if err := r.ParseMultipartForm(0); err != nil {
		return "", &DecodeError{Field: "multipart", Err: err}
	}
	meta := r.MultipartForm.Value["meta"]
	if len(meta) > 1 || len(r.MultipartForm.File["meta"]) != 0 {
		return "", &DecodeError{Field: "meta", Err: errors.New("expected one metadata field")}
	}
	if len(meta) == 0 {
		return "", nil
	}
	var credentials struct {
		Token uniqueToken `json:"token"`
	}
	if err := json.Unmarshal([]byte(meta[0]), &credentials); err != nil {
		return "", &DecodeError{Field: "meta", Err: err}
	}
	return credentials.Token.value, nil
}

type uniqueToken struct {
	value string
	seen  bool
}

func (t *uniqueToken) UnmarshalJSON(data []byte) error {
	if t.seen {
		return errors.New("token must be specified once")
	}
	t.seen = true
	return json.Unmarshal(data, &t.value)
}

func singleToken(values []string) (string, error) {
	if len(values) > 1 {
		return "", &DecodeError{Field: "token", Err: errors.New("expected one token")}
	}
	if len(values) == 0 {
		return "", nil
	}
	return values[0], nil
}
