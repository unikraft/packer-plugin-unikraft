packer {
  required_plugins {
    unikraft = {
      version = ">=v0.2.1"
      source  = "github.com/unikraft/unikraft"
    }
  }
}

source "unikraft-builder" "example" {
  // Platform of the resulting binaries
  architecture = "x86_64"

  // Platform of the resulting binaries
  platform = "qemu"

  // Specific target to build for.
  // Good when there are multiple arch/plat combinations with the same permutations.
  target = "nginx-qemu-x86_64-initrd"

  // Path of the resulting binaries
  build_path = "/tmp/test/.unikraft/apps/nginx"

  // Path where to pull the sources and build the binaries
  workdir = "/tmp/test"

  // Application to pull and build
  pull_source = "app-nginx"

  // If to use the default source manifests
  sources_no_default = false

  // Additional sources to pull
  sources = [ "https://github.com/unikraft/app-nginx.git" ]

  // Log level: trace/debug/info/warn/error/fatal/panic
  log_level = "error"
}

build {
  sources = [
    "source.unikraft-builder.example"
  ]

  post-processor "unikraft-post-processor" {
    // Architecture of the packed binary
    architecture = "x86_64"

    // Platform of the packed binary
    platform = "qemu"

    // Specific target to build for.
    // Good when there are multiple arch/plat combinations with the same permutations.
    // If specified, it will override the given architecture and platform.
    target = "nginx-qemu-x86_64-initrd"

    // Source from where to package
    source = "/tmp/test/.unikraft/apps/nginx"

    // Rootfs to include in the OCI archive
    rootfs = "/tmp/test/.unikraft/apps/nginx/rootfs"

    // The resulting OCI archive
    destination = "my-registry.io/unikernel-nginx:latest"

    // If to push the resulting OCI archive
    push = false

    // Log level: trace/debug/info/warn/error/fatal/panic
    log_level = "error"
  }
}

