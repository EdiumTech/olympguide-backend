variable "cloud_id" {
  type    = string
  default = "b1g3p4bfub6bklcrg72h"
}

variable "folder_id" {
  type    = string
  default = "b1gum9p3p8pqr9jincsb"
}

variable "zone" {
  type    = string
  default = "ru-central1-d"
}

variable "name" {
  type    = string
  default = "olympguide-prod"
}

variable "existing_network_id" {
  description = "Reuse an existing VPC when the cloud's network quota is exhausted. Its settings remain unmanaged."
  type        = string
  default     = null
}

variable "api_domain" {
  type    = string
  default = "api.olympguide.ru"
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9.-]+[a-z0-9]$", var.api_domain))
    error_message = "Use a DNS hostname without a scheme, path or trailing dot."
  }
}

variable "dns_zone_id" {
  description = "Existing public olympguide.ru DNS zone ID. Obtain with yc dns zone list."
  type        = string
}

variable "manage_dns_record" {
  description = "Enable only after reviewing/importing any existing api A record."
  type        = bool
  default     = false
}

variable "ssh_public_key" {
  type = string
  validation {
    condition     = can(regex("^ssh-ed25519 [A-Za-z0-9+/=]+", trimspace(var.ssh_public_key)))
    error_message = "Supply an Ed25519 PUBLIC key. Never put a private key in Terraform."
  }
}

variable "ssh_allowed_cidrs" {
  description = "Administrator public IPv4 addresses, normally /32. Required; no world-open SSH."
  type        = list(string)
  validation {
    condition = length(var.ssh_allowed_cidrs) > 0 && alltrue([
      for cidr in var.ssh_allowed_cidrs : can(cidrnetmask(cidr)) && try(tonumber(split("/", cidr)[1]) >= 24, false)
    ])
    error_message = "Supply one or more IPv4 CIDRs with prefix /24 or narrower."
  }
}

variable "cores" {
  type    = number
  default = 2
}

variable "memory_gb" {
  type    = number
  default = 4
}

variable "data_disk_gb" {
  type    = number
  default = 40
}

variable "snapshot_count" {
  type    = number
  default = 7
}
