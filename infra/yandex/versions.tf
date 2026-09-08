terraform {
  required_version = ">= 1.12.0, < 2.0.0"
  required_providers {
    yandex = {
      source  = "yandex-cloud/yandex"
      version = "0.220.0"
    }
  }
}

# Credentials come from YC_TOKEN or YC_SERVICE_ACCOUNT_KEY_FILE, never tfvars.
provider "yandex" {
  cloud_id  = var.cloud_id
  folder_id = var.folder_id
  zone      = var.zone
}
