locals {
  # Applied to every resource that accepts labels. Label values are restricted
  # to lowercase letters, digits, hyphens and underscores.
  labels = {
    application = "marketplace"
    environment = var.environment
    managed_by  = "terraform"
  }
}
