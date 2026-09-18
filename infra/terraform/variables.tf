variable "project_id" {
  description = "Id of the GCP project this configuration manages."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project_id))
    error_message = "The project id must be a valid GCP project id: 6 to 30 characters, lowercase letters, digits and hyphens, starting with a letter."
  }
}

variable "region" {
  description = "Region every regional resource is created in (docs/requirements.md, section 26)."
  type        = string
  default     = "us-central1"
}

variable "environment" {
  description = "Environment this configuration describes. It is part of every label, so a resource says which environment it belongs to without its project being looked up."
  type        = string

  validation {
    condition     = contains(["lab", "prod"], var.environment)
    error_message = "The environment must be either lab or prod (docs/infrastructure.md)."
  }
}

variable "platform_host" {
  description = "Host the platform answers on. The marketplace serving a request is resolved from its host (docs/requirements.md, section 7), so this is the platform's own host and not a marketplace's; marketplace hosts arrive with F5."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$", var.platform_host))
    error_message = "The platform host must be a lowercase DNS name, with no scheme, no port and no trailing dot."
  }
}

variable "github_repository" {
  description = "Repository allowed to deploy, as owner/name. The federation provider already carries this value in its attribute condition (docs/infrastructure.md); it is repeated here because the binding on the deploy account is Terraform's while the provider is not."
  type        = string
  default     = "aleogr/marketplace"

  validation {
    condition     = can(regex("^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$", var.github_repository))
    error_message = "The repository must be written as owner/name."
  }
}

variable "workload_identity_pool" {
  description = "Id of the Workload Identity Federation pool the bootstrap block created (docs/infrastructure.md). Terraform does not manage the pool; it binds an account to it."
  type        = string
  default     = "github"
}

variable "initial_image" {
  description = "Image the Cloud Run service is created with, and never the image it runs. Terraform cannot create the service without naming one, and on the merge that creates it no image of this repository exists yet: the pipeline builds the first one afterwards. From then on the running image is the pipeline's and Terraform ignores it (see cloud_run.tf)."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}

variable "database_tier" {
  description = "Cloud SQL machine type. The only material recurring cost of the design, which is why it is per-environment rather than a default here (docs/infrastructure.md). Changing it restarts the instance and does not touch the data."
  type        = string
}

variable "database_disk_size" {
  description = "Size in GB of the database disk, which is also its floor: Cloud SQL grows a disk and never shrinks one."
  type        = number
  default     = 10

  validation {
    condition     = var.database_disk_size >= 10
    error_message = "Cloud SQL's minimum disk is 10 GB."
  }
}

variable "database_disk_limit" {
  description = "Ceiling for automatic disk growth. Growth is one-way, so an unbounded limit turns one runaway write into a permanent bill."
  type        = number
  default     = 50
}

variable "database_backup_count" {
  description = "How many automated backups are kept. The design's target of 30 days is production's; an environment that is rebuilt from this repository keeps fewer (docs/infrastructure.md)."
  type        = number
  default     = 7
}

variable "database_log_retention_days" {
  description = "Days of write-ahead log kept for point-in-time recovery, 1 to 7 on Cloud SQL Enterprise. This is the window that costs, because logs are archived every five minutes whether or not anything happened."
  type        = number
  default     = 7

  validation {
    condition     = var.database_log_retention_days >= 1 && var.database_log_retention_days <= 7
    error_message = "Cloud SQL Enterprise keeps between 1 and 7 days of transaction logs."
  }
}
