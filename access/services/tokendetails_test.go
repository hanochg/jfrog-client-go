package services

import (
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

const ownTokenDetailsResponse = `{
  "token_id": "ecb9bfab-57b0-4683-b1a3-1d85acadcae0",
  "subject": "jfac@01epbwg4fcnwfx1v868vmh04t7/users/hanochg",
  "issued_at": 1795600937,
  "issuer": "jfac@01epbwg4fcnwfx1v868vmh04t7",
  "audience": "*@*",
  "description": "",
  "refreshable": false,
  "scope": "applied-permissions/roles:abtest:\"Project Admin\",",
  "project_key": "abtest",
  "last_used": 1795600945
}`

func TestGetOwnTokenDetails_SendsTokenAndDecodesTheResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/access/api/v1/tokens/me", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(ownTokenDetailsResponse))
	}))
	defer server.Close()

	details, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
	require.NoError(t, err)
	issuedAt, lastUsed, refreshable := int64(1795600937), int64(1795600945), false
	assert.Equal(t, &TokenDetails{
		TokenId:     "ecb9bfab-57b0-4683-b1a3-1d85acadcae0",
		Subject:     "jfac@01epbwg4fcnwfx1v868vmh04t7/users/hanochg",
		IssuedAt:    &issuedAt,
		Issuer:      "jfac@01epbwg4fcnwfx1v868vmh04t7",
		Audience:    "*@*",
		Refreshable: &refreshable,
		Scope:       `applied-permissions/roles:abtest:"Project Admin",`,
		ProjectKey:  "abtest",
		LastUsed:    &lastUsed,
	}, details)
}

func TestGetOwnTokenDetails_FieldsAreOptional(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"token_id":"id-1","expiry":1800000000}`))
	}))
	defer server.Close()

	details, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
	require.NoError(t, err)
	expiry := int64(1800000000)
	assert.Equal(t, &TokenDetails{TokenId: "id-1", Expiry: &expiry}, details)
}

func TestGetOwnTokenDetails_AuthResponseIsNotRetried(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		hits := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			w.WriteHeader(code)
		}))
		details, err := newTestTokenService(t, server.URL, 2).GetOwnTokenDetails()
		server.Close()
		var rejected *AuthResponseError
		require.ErrorAs(t, err, &rejected)
		assert.Equal(t, &AuthResponseError{StatusCode: code, Api: "api/v1/tokens/me"}, rejected)
		assert.Nil(t, details)
		assert.Equal(t, 1, hits)
	}
}

func TestGetOwnTokenDetails_UnexpectedStatusIncludesResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"code":"NOT_FOUND","message":"Token not found"}]}`))
	}))
	defer server.Close()

	_, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
	assertResponseError(t, err, http.StatusNotFound)
	assert.ErrorContains(t, err, "Token not found")
}

func TestGetOwnTokenDetails_ServerErrorAfterRetries(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := newTestTokenService(t, server.URL, 2).GetOwnTokenDetails()
	assertResponseError(t, err, http.StatusServiceUnavailable)
	assert.Equal(t, 3, hits)
}

func TestGetOwnTokenDetails_TransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	service := newTestTokenService(t, server.URL, 0)
	server.Close()

	details, err := service.GetOwnTokenDetails()
	assert.Error(t, err)
	assert.Nil(t, details)
}

func TestGetOwnTokenDetails_BodyIsNotAnObject(t *testing.T) {
	for _, body := range []string{"<html>", `["a"]`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		_, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
		server.Close()
		assert.Error(t, err, body)
	}
}

func TestGetOwnTokenDetails_DoesNotFollowRedirects(t *testing.T) {
	server, targetHits := newRedirectingServer(t)
	details, err := newTestTokenService(t, server.URL, 0).GetOwnTokenDetails()
	assertResponseError(t, err, http.StatusFound)
	assert.Nil(t, details)
	assert.Zero(t, *targetHits)
}
