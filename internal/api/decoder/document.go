package decoder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mrbeaver1/dock-service/internal/api/dto"
)

type DecodeError struct {
	Field string
	Err   error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("decode %s: %v", e.Field, e.Err)
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
	query := r.URL.Query()

	login := query.Get("login")
	key := query.Get("key")
	value := query.Get("value")
	limit := query.Get("limit")

	req.Login = &login
	req.Key = &key
	req.Value = &value
	limitInt, err := strconv.ParseUint(limit, 10, 64)

	if err != nil {
		return req, err
	}

	req.Limit = &limitInt

	return req, nil
}
