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
