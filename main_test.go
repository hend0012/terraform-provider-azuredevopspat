package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// TestAzureDevOpsClient tests the AzureDevOpsClient's ability to create a PAT
func TestAzureDevOpsClient(t *testing.T) {
	// Step 1: Set up Azure identity and retrieve a token
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		t.Fatalf("failed to create DefaultAzureCredential: %v", err)
	}

	// Get token with the correct scope for Azure DevOps
	token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"499b84ac-1321-427f-aa17-267ca6975798/.default"},
	})
	if err != nil {
		t.Fatalf("failed to retrieve token: %v", err)
	}

	// Construct the Azure DevOps Client with base URL only
	client, err := NewAzureDevOpsClient("https://vssps.dev.azure.com/", "DevOps-SST", "KPN001", "7.2-preview.1", token.Token)
	if err != nil {
		t.Fatalf("failed to create AzureDevOpsClient: %v", err)
	}

	// Construct the PAT URL explicitly for the API request
	patURL := "https://vssps.dev.azure.com/DevOps-SST/_apis/tokens/pats?api-version=7.2-preview.1"
	fmt.Printf("Constructed PAT Creation URL: %s\n", patURL)

	// Step 2: Attempt to create the PAT by sending a POST request to the specified URL
	displayName := "TestPAT"
	expirationDays := 90
	scope := "app_token"
	allOrganization := false

	payload := map[string]interface{}{
		"displayName": displayName,
		"scope":       scope,
		"validTo":     time.Now().AddDate(0, 0, expirationDays).Format(time.RFC3339),
		"allOrgs":     allOrganization,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", patURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Full Response Body: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API returned non-200 status: %d", resp.StatusCode)
	}

	// Optional: Parse the PAT response if it works
	var patResponse map[string]interface{}
	err = json.Unmarshal(body, &patResponse)
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	fmt.Printf("Created PAT Response: %v\n", patResponse)
}
