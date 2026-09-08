data "yandex_compute_image" "ubuntu" {
  family = "ubuntu-2404-lts"
}

data "yandex_dns_zone" "existing" {
  dns_zone_id = var.dns_zone_id
}

resource "yandex_vpc_network" "backend" {
  count = var.existing_network_id == null ? 1 : 0
  name  = var.name
}

locals {
  network_id = var.existing_network_id != null ? var.existing_network_id : yandex_vpc_network.backend[0].id
}

resource "yandex_vpc_subnet" "backend" {
  name           = var.name
  zone           = var.zone
  network_id     = local.network_id
  v4_cidr_blocks = ["10.42.0.0/24"]
}

resource "yandex_vpc_security_group" "backend" {
  name       = var.name
  network_id = local.network_id

  ingress {
    description    = "HTTPS API"
    protocol       = "TCP"
    port           = 443
    v4_cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    description    = "ACME and redirect to HTTPS"
    protocol       = "TCP"
    port           = 80
    v4_cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    description    = "Administrator SSH"
    protocol       = "TCP"
    port           = 22
    v4_cidr_blocks = var.ssh_allowed_cidrs
  }
  egress {
    description    = "Package repositories, SMTP and external APIs"
    protocol       = "ANY"
    v4_cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "yandex_vpc_address" "backend" {
  name = "${var.name}-ip"
  external_ipv4_address {
    zone_id = var.zone
  }
  lifecycle {
    prevent_destroy = true
  }
}

resource "yandex_compute_disk" "data" {
  name = "${var.name}-data"
  zone = var.zone
  type = "network-ssd"
  size = var.data_disk_gb
  lifecycle {
    prevent_destroy = true
  }
}

resource "yandex_compute_instance" "backend" {
  name                      = var.name
  platform_id               = "standard-v3"
  zone                      = var.zone
  allow_stopping_for_update = true

  resources {
    cores         = var.cores
    memory        = var.memory_gb
    core_fraction = 100
  }
  boot_disk {
    initialize_params {
      image_id = data.yandex_compute_image.ubuntu.id
      size     = 25
      type     = "network-hdd"
    }
  }
  secondary_disk {
    disk_id     = yandex_compute_disk.data.id
    device_name = "olympguide-data"
    auto_delete = false
  }
  network_interface {
    index              = 0
    subnet_id          = yandex_vpc_subnet.backend.id
    nat                = true
    nat_ip_address     = yandex_vpc_address.backend.external_ipv4_address[0].address
    security_group_ids = [yandex_vpc_security_group.backend.id]
  }
  metadata = {
    user-data = templatefile("${path.module}/cloud-init.yaml.tftpl", {
      ssh_public_key = trimspace(var.ssh_public_key)
    })
  }
  lifecycle {
    # An upstream Ubuntu image update must not replace a running VM.
    ignore_changes = [boot_disk[0].initialize_params[0].image_id]
  }
}

resource "yandex_compute_snapshot_schedule" "data" {
  name           = "${var.name}-daily"
  snapshot_count = var.snapshot_count
  disk_ids       = [yandex_compute_disk.data.id]
  schedule_policy {
    expression = "0 2 * * *"
  }
  snapshot_spec {
    description = "Daily data disk snapshot; PostgreSQL dumps are in backups/"
  }
}

resource "yandex_dns_recordset" "api" {
  count   = var.manage_dns_record ? 1 : 0
  zone_id = data.yandex_dns_zone.existing.id
  name    = "${var.api_domain}."
  type    = "A"
  ttl     = 300
  data    = [yandex_vpc_address.backend.external_ipv4_address[0].address]
  lifecycle {
    precondition {
      condition     = data.yandex_dns_zone.existing.public && endswith("${var.api_domain}.", ".${data.yandex_dns_zone.existing.zone}")
      error_message = "The selected DNS zone must be public and contain the API domain."
    }
  }
}
