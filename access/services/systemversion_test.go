package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckSystemVersion_SendsOnlyTheBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/access/api/v1/system/version", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	status, err := newTestSystemService(t, server.URL, 0).CheckSystemVersion()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
}

func TestCheckSystemVersion_RefusesWithoutToken(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	service := newTestSystemService(t, server.URL, 0)
	service.ServiceDetails.SetAccessToken("")

	status, err := service.CheckSystemVersion()
	assert.ErrorContains(t, err, "no access token to check")
	assert.Zero(t, status)
	assert.Zero(t, hits)
}

func TestCheckSystemVersion_ReturnsRejectStatusWithoutRetry(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		hits := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			w.WriteHeader(code)
		}))
		status, err := newTestSystemService(t, server.URL, 3).CheckSystemVersion()
		server.Close()
		require.NoError(t, err)
		assert.Equal(t, code, status)
		assert.Equal(t, 1, hits)
	}
}

func TestCheckSystemVersion_ServerErrorKeepsStatusAfterRetries(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	status, err := newTestSystemService(t, server.URL, 2).CheckSystemVersion()
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, 3, hits)
}

func TestCheckSystemVersion_TransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	service := newTestSystemService(t, server.URL, 0)
	server.Close()

	status, err := service.CheckSystemVersion()
	assert.Error(t, err)
	assert.Zero(t, status)
}

func TestCheckSystemVersion_DoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the redirect was followed")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/elsewhere", http.StatusFound)
	}))
	defer server.Close()

	status, err := newTestSystemService(t, server.URL, 0).CheckSystemVersion()
	assert.ErrorContains(t, err, "redirects aren't followed")
	assert.Zero(t, status)
}
