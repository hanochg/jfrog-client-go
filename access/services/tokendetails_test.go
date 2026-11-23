package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTokenService(t *testing.T, serverUrl string, retries int) *TokenService {
	client, details := newTestServiceDetails(t, serverUrl, retries)
	service := NewTokenService(client)
	service.ServiceDetails = details
	return service
}

func TestGetOwnTokenDetails_SendsTokenAndDecodesDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/access/api/v1/tokens/me", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"token_id":"t1","subject":"jfac@01h/users/alice","scope":"applied-permissions/user","expiry":1700003600}`))
	}))
	defer server.Close()

	status, details, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "jfac@01h/users/alice", details["subject"])
	assert.Equal(t, json.Number("1700003600"), details["expiry"])
}

func TestGetOwnTokenDetails_ReturnsRejectStatusWithoutRetry(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		hits := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"errors":[{"message":"bad"}]}`))
		}))

		status, details, err := newTestTokenService(t, server.URL, 2).GetOwnTokenDetails()
		server.Close()
		require.NoError(t, err)
		assert.Equal(t, code, status)
		assert.Nil(t, details)
		assert.Equal(t, 1, hits)
	}
}

func TestGetOwnTokenDetails_ServerErrorKeepsStatusAfterRetries(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	status, details, err := newTestTokenService(t, server.URL, 2).GetOwnTokenDetails()
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Nil(t, details)
	assert.Equal(t, 3, hits)
}

func TestGetOwnTokenDetails_TransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	service := newTestTokenService(t, server.URL, 0)
	server.Close()

	status, _, err := service.GetOwnTokenDetails()
	assert.Error(t, err)
	assert.Zero(t, status)
}

func TestGetOwnTokenDetails_BodyIsNotAnObject(t *testing.T) {
	for _, body := range []string{"null", "[1]", "not json"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))

		_, details, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
		server.Close()
		assert.ErrorContains(t, err, "api/v1/tokens/me", body)
		assert.Nil(t, details, body)
	}
}

func TestGetOwnTokenDetails_DoesNotFollowRedirects(t *testing.T) {
	targetHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits++
		_, _ = w.Write([]byte(`{"subject":"x"}`))
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer server.Close()

	status, _, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
	assert.ErrorContains(t, err, "redirects aren't followed")
	assert.ErrorContains(t, err, target.URL)
	assert.Zero(t, status)
	assert.Zero(t, targetHits)
}

func TestGetOwnTokenDetails_RedirectStatusWithoutLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	_, _, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
	assert.ErrorContains(t, err, "redirects aren't followed")
}
