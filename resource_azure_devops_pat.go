package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAzureDevOpsPAT() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAzureDevOpsPATCreate,
		ReadContext:   resourceAzureDevOpsPATRead,
		UpdateContext: resourceAzureDevOpsPATUpdate,
		DeleteContext: resourceAzureDevOpsPATDelete,

		Schema: map[string]*schema.Schema{
			"display_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"scope": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "app_token",
			},
			"authorization_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"token": {
				Type:      schema.TypeString,
				Computed:  true,
				Sensitive: true,
			},
			"valid_to": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"renew_before_days": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  7,
			},
			"expiration_days": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  90,
			},
			"all_organization": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"project": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
		},
	}
}

func resourceAzureDevOpsPATCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, ok := meta.(*AzureDevOpsClient)
	if !ok {
		return diag.Errorf("failed to cast meta to *AzureDevOpsClient")
	}

	// Extract renew_before_days and expiration_days from schema
	renewBeforeDays := d.Get("renew_before_days").(int)
	expirationDays := d.Get("expiration_days").(int)

	// Validation check to ensure renew_before_days does not exceed expiration_days
	if renewBeforeDays > expirationDays {
		return diag.Errorf("`renew_before_days` (%d) cannot be greater than `expiration_days` (%d)", renewBeforeDays, expirationDays)
	}

	// Proceed with PAT creation logic
	displayName := d.Get("display_name").(string)
	scope := d.Get("scope").(string)
	allOrgs := d.Get("all_organization").(bool)
	project := d.Get("project").(string)

	pat, err := client.CreatePAT(displayName, scope, expirationDays, allOrgs, project)
	if err != nil {
		return diag.FromErr(err)
	}

	// Set resource data fields
	d.SetId(pat.AuthorizationID)
	d.Set("token", pat.Token)
	d.Set("authorization_id", pat.AuthorizationID)
	d.Set("display_name", pat.DisplayName)
	d.Set("valid_to", pat.ValidTo)

	return resourceAzureDevOpsPATRead(ctx, d, meta)
}

func resourceAzureDevOpsPATRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, ok := meta.(*AzureDevOpsClient)
	if !ok {
		return diag.Errorf("failed to cast meta to *AzureDevOpsClient")
	}

	fmt.Println("Debug: Entering resourceAzureDevOpsPATRead")

	authID := d.Id()
	validToStr := d.Get("valid_to").(string)
	renewBeforeDays := d.Get("renew_before_days").(int)
	expirationDays := d.Get("expiration_days").(int)

	// Check if `expiration_days` is valid compared to `renew_before_days`
	if expirationDays <= renewBeforeDays {
		return diag.Errorf("Invalid configuration: `expiration_days` (%d) must be greater than `renew_before_days` (%d) to prevent renewal loop", expirationDays, renewBeforeDays)
	}

	// Parse `valid_to` date
	validTo, err := time.Parse(time.RFC3339, validToStr)
	if err != nil {
		return diag.Errorf("failed to parse valid_to date: %v", err)
	}

	renewalThreshold := validTo.Add(-time.Duration(renewBeforeDays) * 24 * time.Hour)
	if time.Now().After(renewalThreshold) && time.Now().Before(validTo) {
		fmt.Println("Debug: PAT is within renew window; triggering renewal in Update")
		return resourceAzureDevOpsPATUpdate(ctx, d, meta)
	} else if time.Now().After(validTo) {
		fmt.Println("Debug: PAT is expired; marking for recreation")
		d.SetId("") // Mark resource for recreation
		return nil
	}

	// Attempt to retrieve the PAT
	pat, err := client.GetPAT(authID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			fmt.Printf("Debug: PAT with AuthorizationID=%s not found. Marking for recreation.\n", authID)
			d.SetId("") // Mark resource for recreation
			return nil
		}
		return diag.FromErr(err)
	}

	d.Set("display_name", pat.DisplayName)
	d.Set("valid_to", pat.ValidTo)
	d.Set("authorization_id", pat.AuthorizationID)

	fmt.Println("Debug: Exiting resourceAzureDevOpsPATRead normally")
	return nil
}

func resourceAzureDevOpsPATUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, ok := meta.(*AzureDevOpsClient)
	if !ok {
		return diag.Errorf("failed to cast meta to *AzureDevOpsClient")
	}

	authID := d.Id()
	renewBeforeDays := d.Get("renew_before_days").(int)

	// Parse `valid_to` date
	validTo, err := time.Parse(time.RFC3339, d.Get("valid_to").(string))
	if err != nil {
		return diag.Errorf("failed to parse valid_to date: %v", err)
	}

	// Calculate renewal threshold
	currentTime := time.Now()
	renewalThreshold := validTo.Add(-time.Duration(renewBeforeDays) * 24 * time.Hour)

	// If renewal window is met or valid_to is in the past, recreate the token
	if currentTime.After(renewalThreshold) || currentTime.After(validTo) {
		fmt.Println("PAT is expired or within the renew window. Recreating...")

		// Revoke and recreate PAT if expiration is near or `allOrgs` changed
		if err := client.RevokePAT(authID); err != nil {
			return diag.FromErr(err)
		}
		return resourceAzureDevOpsPATCreate(ctx, d, meta)
	}

	// If `allOrgs` has changed, recreate the PAT (since it is immutable)
	if d.HasChange("all_organization") {
		fmt.Println("allOrgs field has changed. Recreating the PAT.")

		// Revoke the old PAT and create a new one
		if err := client.RevokePAT(authID); err != nil {
			return diag.FromErr(err)
		}
		return resourceAzureDevOpsPATCreate(ctx, d, meta)
	}

	// Handle updates to other mutable fields
	if d.HasChange("display_name") || d.HasChange("scope") || d.HasChange("expiration_days") {
		displayName := d.Get("display_name").(string)
		scope := d.Get("scope").(string)
		expirationDays := d.Get("expiration_days").(int)
		allOrganizations := d.Get("all_organization").(bool)

		_, err := client.UpdatePAT(authID, displayName, scope, expirationDays, allOrganizations)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// Refresh the resource state to reflect any updates
	return resourceAzureDevOpsPATRead(ctx, d, meta)
}

func resourceAzureDevOpsPATDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, ok := meta.(*AzureDevOpsClient)
	if !ok {
		return diag.Errorf("failed to cast meta to *AzureDevOpsClient")
	}

	authID := d.Id()
	if err := client.RevokePAT(authID); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
