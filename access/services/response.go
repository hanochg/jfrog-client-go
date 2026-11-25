package services

import (
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type TokenRejectedError struct {
	StatusCode int
	Api        string
}

func (e *TokenRejectedError) Error() string {
	return fmt.Sprintf("GET %s: the token was rejected: %d %s", e.Api, e.StatusCode, http.StatusText(e.StatusCode))
}

func checkResponse(resp *http.Response, body []byte, api string) error {
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errorutils.CheckError(&TokenRejectedError{StatusCode: resp.StatusCode, Api: api})
	}
	return errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK)
}
