package services

import (
	"encoding/json"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// #nosec G101 -- False positive - no hardcoded credentials.
// jfrog-ignore - not a real token
const ownTokenDetailsApi = tokensApi + "/me"

type TokenDetails struct {
	TokenId     string `json:"token_id,omitempty"`
	Subject     string `json:"subject,omitempty"`
	IssuedAt    *int64 `json:"issued_at,omitempty"`
	Expiry      *int64 `json:"expiry,omitempty"`
	Issuer      string `json:"issuer,omitempty"`
	Audience    string `json:"audience,omitempty"`
	Description string `json:"description,omitempty"`
	Refreshable *bool  `json:"refreshable,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ProjectKey  string `json:"project_key,omitempty"`
	LastUsed    *int64 `json:"last_used,omitempty"`
}

func (ps *TokenService) GetOwnTokenDetails() (*TokenDetails, error) {
	httpDetails := ps.ServiceDetails.CreateHttpClientDetails()
	resp, body, _, err := ps.client.SendGet(ps.ServiceDetails.GetUrl()+ownTokenDetailsApi, false, &httpDetails)
	if resp == nil {
		return nil, err
	}
	if err = checkResponse(resp, body, ownTokenDetailsApi); err != nil {
		return nil, err
	}
	var details TokenDetails
	err = json.Unmarshal(body, &details)
	return &details, errorutils.CheckError(err)
}
