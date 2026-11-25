package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	accessAuth "github.com/jfrog/jfrog-client-go/access/auth"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServiceDetails(t *testing.T, serverUrl string, retries int) (*jfroghttpclient.JfrogHttpClient, auth.ServiceDetails) {
	t.Helper()
	client, err := jfroghttpclient.JfrogClientBuilder().SetRetries(retries).SetRetryWaitMilliSecs(0).Build()
	require.NoError(t, err)
	details := accessAuth.NewAccessDetails()
	details.SetUrl(serverUrl + "/access/")
	details.SetAccessToken("tok")
	return client, details
}

func newTestSystemService(t *testing.T, serverUrl string, retries int) *SystemService {
	client, details := newTestServiceDetails(t, serverUrl, retries)
	service := NewSystemService(client)
	service.ServiceDetails = details
	return service
}

const artifactoryVersionResponse = `{
  "version": "7.179.0",
  "revision": "717900900",
  "servicesVersions": {"package_handler_version": "5.1075.0"},
  "addons": ["aol", "ha", "docker"],
  "license": "f2cabf4612706b0844e2381c158207adafbc8df64",
  "entitlements": {"EVENT_BASED_PULL_REPLICATION": true, "REPO_REPLICATION": true}
}`

func newRedirectingServer(t *testing.T) (*httptest.Server, *int) {
	targetHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL+"/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)
	return server, &targetHits
}

func assertResponseError(t *testing.T, err error, statusCode int) *errorutils.HttpResponseError {
	t.Helper()
	var responseErr *errorutils.HttpResponseError
	require.ErrorAs(t, err, &responseErr)
	assert.Equal(t, statusCode, responseErr.StatusCode)
	return responseErr
}

func TestGetRootCertificate_NoAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/access/api/v1/cert/root", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("QUJD\n"))
	}))
	defer server.Close()

	cert, err := newTestSystemService(t, server.URL, 0).GetRootCertificate()
	require.NoError(t, err)
	assert.Equal(t, "QUJD", cert)
}

func TestGetRootCertificate_Non200IncludesResponseBody(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"errors":[{"message":"no certificate"}]}`))
		}))
		_, err := newTestSystemService(t, server.URL, 0).GetRootCertificate()
		server.Close()
		assertResponseError(t, err, code)
		assert.ErrorContains(t, err, "no certificate")
	}
}

func TestGetRootCertificate_ServerErrorAfterRetries(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 2).GetRootCertificate()
	assertResponseError(t, err, http.StatusServiceUnavailable)
	assert.Equal(t, 3, hits)
}

func TestGetRootCertificate_DoesNotFollowRedirects(t *testing.T) {
	server, targetHits := newRedirectingServer(t)
	_, err := newTestSystemService(t, server.URL, 0).GetRootCertificate()
	assertResponseError(t, err, http.StatusFound)
	assert.Zero(t, *targetHits)
}

func TestGetArtifactoryVersion_SendsOnlyTheBearerTokenAndDecodesTheResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/artifactory/api/system/version", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(artifactoryVersionResponse))
	}))
	defer server.Close()

	version, err := newTestSystemService(t, server.URL, 0).GetArtifactoryVersion()
	require.NoError(t, err)
	assert.Equal(t, &ArtifactoryVersion{
		Version:          "7.179.0",
		Revision:         "717900900",
		ServicesVersions: map[string]string{"package_handler_version": "5.1075.0"},
		Addons:           []string{"aol", "ha", "docker"},
		License:          "f2cabf4612706b0844e2381c158207adafbc8df64",
		Entitlements:     map[string]bool{"EVENT_BASED_PULL_REPLICATION": true, "REPO_REPLICATION": true},
	}, version)
}

func TestGetArtifactoryVersion_FieldsAreOptional(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	version, err := newTestSystemService(t, server.URL, 0).GetArtifactoryVersion()
	require.NoError(t, err)
	assert.Equal(t, &ArtifactoryVersion{}, version)
}

func TestGetArtifactoryVersion_RefusesWithoutToken(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
	}))
	defer server.Close()
	service := newTestSystemService(t, server.URL, 0)
	service.ServiceDetails.SetAccessToken("")

	version, err := service.GetArtifactoryVersion()
	assert.ErrorContains(t, err, "no access token to send")
	assert.Nil(t, version)
	assert.Zero(t, hits)
}

func TestGetArtifactoryVersion_RefusesAnAccessUrlWithoutAccessPath(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
	}))
	defer server.Close()
	service := newTestSystemService(t, server.URL, 0)
	service.ServiceDetails.SetUrl(server.URL + "/other/")

	_, err := service.GetArtifactoryVersion()
	assert.ErrorContains(t, err, "doesn't end with access/")
	assert.Zero(t, hits)
}

func TestGetArtifactoryVersion_AuthResponseIsNotRetried(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		hits := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			w.WriteHeader(code)
		}))
		version, err := newTestSystemService(t, server.URL, 3).GetArtifactoryVersion()
		server.Close()
		var rejected *AuthResponseError
		require.ErrorAs(t, err, &rejected)
		assert.Equal(t, &AuthResponseError{StatusCode: code, Api: "artifactory/api/system/version"}, rejected)
		assert.Nil(t, version)
		assert.Equal(t, 1, hits)
	}
}

func TestGetArtifactoryVersion_UnexpectedStatusIncludesResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Not Found"}]}`))
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 0).GetArtifactoryVersion()
	assertResponseError(t, err, http.StatusNotFound)
	assert.ErrorContains(t, err, `"message": "Not Found"`)
}

func TestGetArtifactoryVersion_ServerErrorAfterRetries(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 2).GetArtifactoryVersion()
	assertResponseError(t, err, http.StatusServiceUnavailable)
	assert.Equal(t, 3, hits)
}

func TestGetArtifactoryVersion_TransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	service := newTestSystemService(t, server.URL, 0)
	server.Close()

	version, err := service.GetArtifactoryVersion()
	assert.Error(t, err)
	assert.Nil(t, version)
}

func TestGetArtifactoryVersion_BodyIsNotJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>maintenance</html>"))
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 0).GetArtifactoryVersion()
	assert.Error(t, err)
}

func TestGetArtifactoryVersion_DoesNotFollowRedirects(t *testing.T) {
	server, targetHits := newRedirectingServer(t)
	version, err := newTestSystemService(t, server.URL, 0).GetArtifactoryVersion()
	assertResponseError(t, err, http.StatusFound)
	assert.Nil(t, version)
	assert.Zero(t, *targetHits)
}
