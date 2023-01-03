source "kraft-builder" "basic-example" {
  mock = "mock-config"
}

build {
  sources = [
    "source.kraft-builder.basic-example"
  ]

  provisioner "shell-local" {
    inline = [
      "echo build generated data: ${build.GeneratedMockData}",
    ]
  }
}
