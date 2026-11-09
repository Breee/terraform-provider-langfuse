terraform {
  required_providers {
    langfuse = {
      source = "langfuse/langfuse" # Local development override, not a registry install.
    }
  }
}

variable "langfuse_host" {
  type        = string
  description = "Base URL of your Langfuse instance."
}

provider "langfuse" {
  host = var.langfuse_host
  # Organization management reads the admin key from LANGFUSE_ADMIN_KEY.
}
