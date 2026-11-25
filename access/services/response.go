package services

import (
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type AuthResponseError struct {
	StatusCode int
	Api        string
}

func (e *AuthResponseError) Error() string {
	return fmt.Sprintf("GET %s: authentication or authorization failed: %d %s", e.Api, e.StatusCode, http.StatusText(e.StatusCode))
}

func checkResponse(resp *http.Response, body []byte, api string) error {
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errorutils.CheckError(&AuthResponseError{StatusCode: resp.StatusCode, Api: api})
	}
	return errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK)
}
