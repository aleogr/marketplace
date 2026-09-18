output "container_repository" {
  description = "Prefix of every image pushed by the pipeline; the image name is appended to it."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.containers.repository_id}"
}
