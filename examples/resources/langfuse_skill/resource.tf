variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

resource "langfuse_skill" "example" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "release-notes"
  commit_message     = "Initial version"
  labels             = ["production"]
  tags               = ["docs"]

  # Use file("${path.module}/skill/SKILL.md") to load files from disk.
  files = {
    "SKILL.md"            = <<-EOT
      ---
      name: release-notes
      description: Write release notes from a list of merged changes.
      ---
      # Release notes

      Group the changes by type and follow references/style.md.
    EOT
    "references/style.md" = "Use short, imperative sentences.\n"
  }
}
