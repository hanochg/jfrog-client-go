package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	accessAuth "github.com/jfrog/jfrog-client-go/access/auth"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
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

func TestGetRootCertificate_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 0).GetRootCertificate()
	assert.ErrorContains(t, err, "404")
	assert.ErrorContains(t, err, "api/v1/cert/root")
}

func TestGetRootCertificate_ServerErrorAfterRetries(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 2).GetRootCertificate()
	assert.ErrorContains(t, err, "503")
	assert.ErrorContains(t, err, "api/v1/cert/root")
	assert.Equal(t, 3, hits)
}

func TestGetRootCertificate_DoesNotFollowRedirects(t *testing.T) {
	targetHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits++
		_, _ = w.Write([]byte("QUJD"))
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL+"/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 0).GetRootCertificate()
	assert.ErrorContains(t, err, "redirects aren't followed")
	assert.ErrorContains(t, err, target.URL)
	assert.ErrorContains(t, err, "api/v1/cert/root")
	assert.Zero(t, targetHits)
}

func TestGetRootCertificate_RedirectStatusWithoutLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 0).GetRootCertificate()
	assert.ErrorContains(t, err, "redirects aren't followed")
}

func TestGetRootCertificate_ErrorOmitsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("\x1b[31mspoofed body"))
	}))
	defer server.Close()

	_, err := newTestSystemService(t, server.URL, 0).GetRootCertificate()
	assert.ErrorContains(t, err, "404")
	assert.NotContains(t, err.Error(), "spoofed body")
	assert.NotContains(t, err.Error(), "\x1b")
	assert.NotContains(t, err.Error(), "server response")
}

func TestCheckArtifactoryVersion_SendsOnlyTheBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/artifactory/api/system/version", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	status, err := newTestSystemService(t, server.URL, 0).CheckArtifactoryVersion()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
}

func TestCheckArtifactoryVersion_RefusesWithoutToken(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	service := newTestSystemService(t, server.URL, 0)
	service.ServiceDetails.SetAccessToken("")

	status, err := service.CheckArtifactoryVersion()
	assert.ErrorContains(t, err, "no access token to check")
	assert.Zero(t, status)
	assert.Zero(t, hits)
}

func TestCheckArtifactoryVersion_ReturnsRejectStatusWithoutRetry(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		hits := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			w.WriteHeader(code)
		}))
		status, err := newTestSystemService(t, server.URL, 3).CheckArtifactoryVersion()
		server.Close()
		require.NoError(t, err)
		assert.Equal(t, code, status)
		assert.Equal(t, 1, hits)
	}
}

func TestCheckArtifactoryVersion_ServerErrorKeepsStatusAfterRetries(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	status, err := newTestSystemService(t, server.URL, 2).CheckArtifactoryVersion()
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, 3, hits)
}

func TestCheckArtifactoryVersion_TransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	service := newTestSystemService(t, server.URL, 0)
	server.Close()

	status, err := service.CheckArtifactoryVersion()
	assert.Error(t, err)
	assert.Zero(t, status)
}

func TestCheckArtifactoryVersion_DoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the redirect was followed")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL+"/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	status, err := newTestSystemService(t, server.URL, 0).CheckArtifactoryVersion()
	assert.ErrorContains(t, err, "redirects aren't followed")
	assert.Zero(t, status)
}

func TestCheckArtifactoryVersion_RefusesAnAccessUrlWithoutAccessPath(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	service := newTestSystemService(t, server.URL, 0)
	service.ServiceDetails.SetUrl(server.URL + "/other/")

	status, err := service.CheckArtifactoryVersion()
	assert.ErrorContains(t, err, "doesn't end with access/")
	assert.Zero(t, status)
	assert.Zero(t, hits)
}
