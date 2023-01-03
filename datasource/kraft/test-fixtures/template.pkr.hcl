data "kraft-datasource" "test" {
  mock = "mock-config"
}

locals {
  foo = data.kraft-datasource.test.foo
  bar = data.kraft-datasource.test.bar
}

source "null" "basic-example" {
  communicator = "none"
}

build {
  sources = [
    "source.null.basic-example"
  ]

  provisioner "shell-local" {
    inline = [
      "echo foo: ${local.foo}",
      "echo bar: ${local.bar}",
    ]
  }
}
