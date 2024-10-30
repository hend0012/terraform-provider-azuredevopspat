package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AZURE_CLIENT_ID", ""),
			},
			"client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AZURE_CLIENT_SECRET", ""),
			},
			"tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AZURE_TENANT_ID", ""),
			},
			"subscription_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AZURE_SUBSCRIPTION_ID", ""),
			},
			"organization": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The Azure DevOps organization name.",
			},
			"project": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The Azure DevOps project name.",
			},
			"api_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "7.2-preview.1",
				Description: "The API version for Azure DevOps REST calls.",
			},
			"devops_base_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "https://vssps.dev.azure.com",
				Description: "The base URL for Azure DevOps.",
			},
		},
		ConfigureContextFunc: providerConfigure,
		ResourcesMap: map[string]*schema.Resource{
			"azuredevops_pat": resourceAzureDevOpsPAT(),
		},
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Retrieve values from the provider configuration
	organization := d.Get("organization").(string)
	project := d.Get("project").(string)
	apiVersion := d.Get("api_version").(string)
	devopsBaseURL := d.Get("devops_base_url").(string)

	// Instantiate AzureDevOpsClient; let it manage its own token retrieval
	client, err := NewAzureDevOpsClient(devopsBaseURL, organization, project, apiVersion, "")
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Error creating Azure DevOps client",
			Detail:   err.Error(),
		})
		return nil, diags
	}

	return client, diags
}
