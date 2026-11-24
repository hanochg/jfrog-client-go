package services

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// #nosec G101 -- False positive - no hardcoded credentials.
// jfrog-ignore - not a real token
const ownTokenDetailsApi = tokensApi + "/me"

func (ps *TokenService) GetOwnTokenDetails() (int, map[string]interface{}, error) {
	httpDetails := ps.ServiceDetails.CreateHttpClientDetails()
	resp, body, redirectUrl, err := ps.client.SendGet(ps.ServiceDetails.GetUrl()+ownTokenDetailsApi, false, &httpDetails)
	if redirectErr := redirectError(resp, redirectUrl); redirectErr != nil {
		return 0, nil, errorutils.CheckErrorf("GET %s: %s", ownTokenDetailsApi, redirectErr.Error())
	}
	if resp == nil {
		return 0, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var details map[string]interface{}
	if err = decoder.Decode(&details); err != nil || details == nil {
		return resp.StatusCode, nil, errorutils.CheckErrorf("GET %s: the response is not a JSON object", ownTokenDetailsApi)
	}
	return resp.StatusCode, details, nil
}
