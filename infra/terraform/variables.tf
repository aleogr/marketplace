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
