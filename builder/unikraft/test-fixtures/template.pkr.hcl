source "unikraft-builder" "example" {
  // Platform of the resulting binaries
  architecture = "x86_64"

  // Platform of the resulting binaries
  platform = "qemu"

  // Path of the resulting binaries
  build_path = "/tmp/test/.unikraft/apps/helloworld"

  // Path where to pull the sources and build the binaries
  workdir = "/tmp/test"

  // Application to pull and build
  pull_source = "app-helloworld"

  // If to use the default source manifests
  sources_no_default = false

  // Additional sources to pull
  sources = [ "https://github.com/unikraft/app-helloworld.git" ]

  // Log level: trace/debug/info/warn/error/fatal/panic
  log_level = "debug"
}

build {
  sources = [ "source.unikraft-builder.example" ]
}
