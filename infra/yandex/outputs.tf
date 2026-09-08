output "public_ip" {
  value = yandex_vpc_address.backend.external_ipv4_address[0].address
}

output "instance_id" {
  value = yandex_compute_instance.backend.id
}

output "data_disk_id" {
  value = yandex_compute_disk.data.id
}

output "api_url" {
  value = "https://${var.api_domain}/api/v1"
}

output "ssh_user" {
  value = "deploy"
}
