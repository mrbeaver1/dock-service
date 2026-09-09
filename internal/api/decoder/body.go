package decoder

import "net/http"

func finishBody(r *http.Request, parseErr error) error {
	body, ok := r.Body.(interface{ Finish() error })
	if !ok {
		return parseErr
	}
	if parseErr != nil {
		_ = r.Body.Close()
		return parseErr
	}
	return body.Finish()
}
