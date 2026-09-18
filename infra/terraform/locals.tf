locals {
  # Applied to every resource that accepts labels. Label values are restricted
  # to lowercase letters, digits, hyphens and underscores.
  labels = {
    application = "marketplace"
    environment = var.environment
    managed_by  = "terraform"
  }
}

locals {
  # Every marketplace host, flattened, because Cloud Run maps one domain at a
  # time and its mapping supports no wildcard (docs/requirements.md, section 7).
  marketplace_hosts = toset(flatten([
    for marketplace in var.marketplaces : marketplace.hosts
  ]))
}
