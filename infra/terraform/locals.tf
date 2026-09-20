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

locals {
  # ONE PLACE, because three files read it and a fourth will. A connection name
  # is `<project>:<region>:<instance>` and it is assembled rather than typed,
  # so a change to any of the three reaches every reader at once.
  #
  # IT IS ASSEMBLED AND NOT READ FROM THE SHARED STATE, which would make every
  # plan here need that state. The price is that `var.region` here has to agree
  # with `var.region` there: if they ever diverge this names an instance that
  # does not exist, and the migration job says so at the next deployment.
  database_instance = "${var.shared_project_id}:${var.region}:${var.shared_instance}"
}
