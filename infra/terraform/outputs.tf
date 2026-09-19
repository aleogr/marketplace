output "container_repository" {
  description = "Prefix of every image pushed by the pipeline; the image name is appended to it."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.containers.repository_id}"
}

output "service_name" {
  description = "Name of the Cloud Run service the pipeline rolls forward."
  value       = google_cloud_run_v2_service.marketplace.name
}

output "service_url" {
  description = "The run.app URL, which answers as soon as a revision is serving. The pipeline verifies the deployment against this rather than against the custom domain, whose certificate can take a day to be issued (docs/requirements.md, section 7)."
  value       = google_cloud_run_v2_service.marketplace.uri
}

output "platform_url" {
  description = "The host the platform is reached on once its certificate is issued."
  value       = "https://${google_cloud_run_domain_mapping.platform.name}"
}

output "deployer_service_account" {
  description = "Account the deployment pipeline federates into. Its value belongs in the repository variable GCP_DEPLOYER_SA (docs/infrastructure.md)."
  value       = google_service_account.deployer.email
}

output "database_instance" {
  description = "Connection name of the Cloud SQL instance, as the connector and `gcloud sql connect` name it."
  value       = google_sql_database_instance.main.connection_name
}

output "migration_job" {
  description = "Name of the Cloud Run Job the pipeline executes before a revision receives traffic."
  value       = google_cloud_run_v2_job.migrate.name
}

output "mail_webhook_url" {
  description = "Where the e-mail provider posts what became of each message. It is typed into the provider's console by hand, with the token from the `mail-webhook-token` secret (docs/roadmap.md, F11)."
  value       = local.mail_webhook_url
}
