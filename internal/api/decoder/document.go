package decoder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/api/dto"
)

type DecodeError struct {
	Field string
	Err   error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("decode %s: %v", e.Field, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

func DocumentID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, &DecodeError{Field: "id", Err: fmt.Errorf("must be a UUID: %w", err)}
	}
	return id, nil
}

func UploadRequestToDto(r *http.Request) (req dto.CreateDocumentRequest, closeFile func() error, err error) {
	form := r.MultipartForm

	if len(form.Value["meta"]) != 0 {
		var meta *dto.DocumentMeta
		if err := json.Unmarshal([]byte(form.Value["meta"][0]), &meta); err != nil {
			return req, nil, &DecodeError{Field: "meta", Err: err}
		}

		req.Meta = meta
	}

	if len(form.Value["json"]) != 0 {
		req.JSON = json.RawMessage(form.Value["json"][0])
	}

	if files := form.File["file"]; len(files) > 0 {
		header := files[0]
		file, err := header.Open()
		if err != nil {
			return req, nil, fmt.Errorf("open uploaded file: %w", err)
		}
		req.File = &dto.UploadedFile{
			Filename: header.Filename,
			Size:     header.Size,
			Content:  file,
		}
		closeFile = file.Close
	}

	return req, closeFile, nil
}

func GetListRequestToDto(r *http.Request) (req dto.GetDocumentsRequest, err error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return req, &DecodeError{Field: "query", Err: err}
	}
	for field, target := range map[string]**string{"login": &req.Login, "key": &req.Key, "value": &req.Value} {
		values, ok := query[field]
		if !ok {
			continue
		}
		if len(values) != 1 {
			return req, &DecodeError{Field: field, Err: fmt.Errorf("must be provided once")}
		}
		*target = &values[0]
	}
	if values, ok := query["limit"]; ok {
		if len(values) != 1 {
			return req, &DecodeError{Field: "limit", Err: fmt.Errorf("must be provided once")}
		}
		limit, err := strconv.ParseUint(values[0], 10, 64)
		if err != nil {
			return req, &DecodeError{Field: "limit", Err: fmt.Errorf("must be an unsigned integer")}
		}
		req.Limit = &limit
	}
	return req, nil
}
