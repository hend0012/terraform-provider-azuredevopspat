# terraform-provider-azuredevopspat
A provider to manage Azure Devops Tokens



resource "azuredevops_pat" "example_pat1" {
  display_name      = "example-pat"
  scope             = "app_token"
  renew_before_days = 1  # Number of days before expiration to renew the PAT
  expiration_days   = 2
}
