# Azure DevOps PAT Provider

The **Azure DevOps PAT Provider** is a Terraform provider for managing Personal Access Tokens (PATs) in Azure DevOps. This provider supports creating, updating, and revoking PATs with customizable expiration and renewal options, ideal for automating Azure DevOps workflows securely.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) 0.13 or later
- Azure DevOps organization with API access

## Installation

To use this provider, add the following to your Terraform configuration:

```hcl
terraform {
  required_providers {
    azuredevopspat = {
      source = "your-registry/azuredevopspat"
      version = "1.0.0"
    }
  }
}
```
Run terraform init to download the provider.

Provider Configuration

The provider can be configured with the following parameters:
```hcl
provider "azuredevopspat" {
  organization = "DevOps-SST"
  project      = "YourProjectName"
  api_version  = "7.2-preview.1"
}
```
•	organization: The Azure DevOps organization name.
•	project (optional): The Azure DevOps project name.
•	api_version (optional): The Azure DevOps REST API version, defaulting to 7.1-preview.1.

Resource: azuredevops_pat

The azuredevops_pat resource manages Personal Access Tokens (PATs) in Azure DevOps.

Example Usage
```hcl
resource "azuredevopspat" "example_pat" {
  display_name      = "example-pat"
  scope             = "app_token"
  renew_before_days = 7
  expiration_days   = 30
  all_organization  = true
}
```
Argument Reference

•	display_name (Required): The name of the PAT for easy identification.
•	scope (Required): The scope of the PAT. Common values are app_token or more specific scopes such as vso.analytics.
•	renew_before_days (Optional): Number of days before the PAT expiration to trigger renewal.
•	expiration_days (Required): Number of days for the token’s validity period.
•	all_organization (Optional): Boolean indicating if the PAT applies to all organizations. Note: This cannot be updated directly; tokens with this option must be recreated if changed.
•	token (Computed, Sensitive): The generated PAT token value.
•	authorization_id (Computed): Unique authorization ID for the PAT.
•	valid_to (Computed): Expiration date of the token.

Example with Renewal Check

This configuration checks daily for renewal:
```hcl
resource "azuredevopspat" "daily_pat" {
  display_name      = "daily-pat"
  scope             = "app_token"
  renew_before_days = 1
  expiration_days   = 2
  all_organization  = false
}
```
In this case, Terraform will renew the PAT every day by revoking and recreating it if the token is near expiration.

Importing an Existing PAT
```hcl
terraform import azuredevopspat.example_pat <authorization_id>
```
To import an existing PAT into Terraform, use the following command with its authorization ID:

Example Configuration with Multiple PATs

You can also define multiple PAT resources:
```hcl
resource "azuredevopspat" "pat_analytics" {
  display_name      = "analytics-pat"
  scope             = "vso.analytics"
  renew_before_days = 7
  expiration_days   = 90
}

resource "azuredevopspat" "pat_code" {
  display_name      = "code-pat"
  scope             = "vso.code"
  renew_before_days = 30
  expiration_days   = 180
}
```
License

This project is licensed under the MIT License - see the LICENSE file for details.
--- 

You can copy this content into your own `README.md` file. Let me know if you need further assistance! |oai:code-citation|
