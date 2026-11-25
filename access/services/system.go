package services

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
)

const (
	rootCertificateApi    = "api/v1/cert/root"
	accessPath            = "access/"
	artifactoryVersionApi = "artifactory/api/system/version"
)

type SystemService struct {
	client         *jfroghttpclient.JfrogHttpClient
	ServiceDetails auth.ServiceDetails
}

type ArtifactoryVersion struct {
	Version          string            `json:"version,omitempty"`
	Revision         string            `json:"revision,omitempty"`
	ServicesVersions map[string]string `json:"servicesVersions,omitempty"`
	Addons           []string          `json:"addons,omitempty"`
	License          string            `json:"license,omitempty"`
	Entitlements     map[string]bool   `json:"entitlements,omitempty"`
}

func NewSystemService(client *jfroghttpclient.JfrogHttpClient) *SystemService {
	return &SystemService{client: client}
}

func (ss *SystemService) GetRootCertificate() (string, error) {
	anonymous := httputils.HttpClientDetails{}
	resp, body, _, err := ss.client.SendGet(ss.ServiceDetails.GetUrl()+rootCertificateApi, false, &anonymous)
	if resp == nil {
		return "", err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// The version API also answers anonymous requests, so it's never called without the token.
func (ss *SystemService) GetArtifactoryVersion() (*ArtifactoryVersion, error) {
	accessToken := ss.ServiceDetails.GetAccessToken()
	if accessToken == "" {
		return nil, errorutils.CheckErrorf("GET %s: no access token to send", artifactoryVersionApi)
	}
	platformUrl, ok := strings.CutSuffix(ss.ServiceDetails.GetUrl(), accessPath)
	if !ok {
		return nil, errorutils.CheckErrorf("GET %s: the Access URL %q doesn't end with %s, so the platform URL is unknown", artifactoryVersionApi, ss.ServiceDetails.GetUrl(), accessPath)
	}
	httpDetails := httputils.HttpClientDetails{AccessToken: accessToken}
	resp, body, _, err := ss.client.SendGet(platformUrl+artifactoryVersionApi, false, &httpDetails)
	if resp == nil {
		return nil, err
	}
	if err = checkResponse(resp, body, artifactoryVersionApi); err != nil {
		return nil, err
	}
	var version ArtifactoryVersion
	err = json.Unmarshal(body, &version)
	return &version, errorutils.CheckError(err)
}
