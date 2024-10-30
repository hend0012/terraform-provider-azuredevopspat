package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type AzureDevOpsClient struct {
	httpClient   *http.Client
	baseURL      string
	organization string
	apiVersion   string
	token        string
}

type PATToken struct {
	DisplayName     string   `json:"displayName"`
	ValidTo         string   `json:"validTo"`
	Scope           string   `json:"scope"`
	TargetAccounts  []string `json:"targetAccounts"`
	ValidFrom       string   `json:"validFrom"`
	AuthorizationID string   `json:"authorizationId"`
	Token           string   `json:"token"`
}

type PATTokenResponse struct {
	PATToken      PATToken `json:"patToken"`
	PATTokenError string   `json:"patTokenError"`
}

func NewAzureDevOpsClient(devopsBaseURL, organization, project, apiVersion, bearerToken string) (*AzureDevOpsClient, error) {
	if bearerToken == "" {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create DefaultAzureCredential: %v", err)
		}
		token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
			Scopes: []string{"499b84ac-1321-427f-aa17-267ca6975798/.default"},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get token: %v", err)
		}
		bearerToken = token.Token
	}

	if !strings.HasSuffix(devopsBaseURL, "/") {
		devopsBaseURL += "/"
	}
	baseURL := fmt.Sprintf("%s%s/_apis", devopsBaseURL, organization)

	return &AzureDevOpsClient{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		baseURL:      baseURL,
		organization: organization,
		apiVersion:   apiVersion,
		token:        bearerToken,
	}, nil
}

func (c *AzureDevOpsClient) UpdatePAT(authorizationID, displayName, scope string, expirationDays int, allOrganizations bool) (*PATToken, error) {
	url := fmt.Sprintf("%s/tokens/pats?api-version=%s", c.baseURL, c.apiVersion)

	payload := map[string]interface{}{
		"authorizationId": authorizationID,
		"displayName":     displayName,
		"scope":           scope,
		"validTo":         time.Now().AddDate(0, 0, expirationDays).Format(time.RFC3339),
		"allOrgs":         allOrganizations,
	}

	bodyData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %v", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(bodyData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update PAT: status code %d, response: %s", resp.StatusCode, body)
	}

	var patResponse struct {
		PATToken PATToken `json:"patToken"`
	}
	if err := json.Unmarshal(body, &patResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &patResponse.PATToken, nil
}

func (c *AzureDevOpsClient) CreatePAT(displayName, scope string, expirationDays int, allOrganizations bool, project string) (*PATToken, error) {
	url := fmt.Sprintf("%s/tokens/pats?api-version=%s", c.baseURL, c.apiVersion)

	payload := map[string]interface{}{
		"displayName": displayName,
		"scope":       scope,
		"validTo":     time.Now().AddDate(0, 0, expirationDays).Format(time.RFC3339),
		"allOrgs":     allOrganizations,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	var patResponse struct {
		PATToken PATToken `json:"patToken"`
	}
	if err := json.Unmarshal(responseBody, &patResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PAT response: %v", err)
	}

	return &patResponse.PATToken, nil
}

func (c *AzureDevOpsClient) RevokePAT(authorizationID string) error {
	url := fmt.Sprintf("%s/tokens/pats?authorizationId=%s&api-version=%s", c.baseURL, authorizationID, c.apiVersion)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNoContent {
		return nil
	} else if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("API returned status %d: check if DELETE is supported for this endpoint", resp.StatusCode)
	} else if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: check token or authentication method")
	}

	return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, body)
}

func (c *AzureDevOpsClient) GetPAT(authorizationID string) (*PATToken, error) {
	url := fmt.Sprintf("%s/tokens/pats?api-version=%s", c.baseURL, c.apiVersion)

	const maxRetries = 3
	const delay = 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to retrieve PAT: status code %d", resp.StatusCode)
		}

		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var patResponse struct {
			ContinuationToken string     `json:"continuationToken"`
			PATTokens         []PATToken `json:"patTokens"`
		}
		if err := json.Unmarshal(responseBody, &patResponse); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PAT response: %v", err)
		}

		for _, pat := range patResponse.PATTokens {
			if pat.AuthorizationID == authorizationID {
				return &pat, nil
			}
		}

		time.Sleep(delay)
	}

	return nil, fmt.Errorf("PAT with authorization ID %s not found after retries", authorizationID)
}
