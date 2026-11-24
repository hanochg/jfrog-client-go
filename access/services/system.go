package services

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
)

const (
	rootCertificateApi = "api/v1/cert/root"
	readinessApi       = "api/v1/system/readiness"
)

type SystemService struct {
	client         *jfroghttpclient.JfrogHttpClient
	ServiceDetails auth.ServiceDetails
}

func NewSystemService(client *jfroghttpclient.JfrogHttpClient) *SystemService {
	return &SystemService{client: client}
}

func (ss *SystemService) GetRootCertificate() (string, error) {
	anonymous := httputils.HttpClientDetails{}
	resp, body, redirectUrl, err := ss.client.SendGet(ss.ServiceDetails.GetUrl()+rootCertificateApi, false, &anonymous)
	if redirectErr := redirectError(resp, redirectUrl); redirectErr != nil {
		return "", errorutils.CheckErrorf("GET %s: %s", rootCertificateApi, redirectErr.Error())
	}
	if resp == nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errorutils.CheckErrorf("GET %s: unexpected status %d", rootCertificateApi, resp.StatusCode)
	}
	return strings.TrimSpace(string(body)), nil
}

// Readiness answers 200 to anonymous requests too, so it's only asked with an access token, and only that token is sent.
func (ss *SystemService) CheckReadiness() (int, error) {
	accessToken := ss.ServiceDetails.GetAccessToken()
	if accessToken == "" {
		return 0, errorutils.CheckErrorf("GET %s: no access token to check", readinessApi)
	}
	httpDetails := httputils.HttpClientDetails{AccessToken: accessToken}
	resp, _, redirectUrl, err := ss.client.SendGet(ss.ServiceDetails.GetUrl()+readinessApi, false, &httpDetails)
	if redirectErr := redirectError(resp, redirectUrl); redirectErr != nil {
		return 0, errorutils.CheckErrorf("GET %s: %s", readinessApi, redirectErr.Error())
	}
	if resp == nil {
		return 0, err
	}
	return resp.StatusCode, nil
}

// Redirects aren't followed: Go forwards the Authorization header to any port or scheme on the same host and drops it
// for other hosts, so following one could send the token in cleartext or turn the check into an anonymous request.
func redirectError(resp *http.Response, redirectUrl string) error {
	if redirectUrl == "" && (resp == nil || resp.StatusCode < 300 || resp.StatusCode >= 400) {
		return nil
	}
	if redirectUrl == "" {
		redirectUrl = resp.Header.Get("Location")
	}
	return fmt.Errorf("the server redirected the request to %q, and redirects aren't followed; configure the server's final URL", redirectUrl)
}
