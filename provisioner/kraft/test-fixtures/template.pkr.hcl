source "null" "basic-example" {
  communicator = "none"
}

build {
  sources = [
    "source.null.basic-example"
  ]

  provisioner "kraft-provisioner" {
    mock = "my-mock-config"
  }
}
