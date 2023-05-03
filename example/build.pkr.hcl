packer {
  required_plugins {
    unikraft = {
      version = ">=v0.1.0"
      source  = "github.com/unikraft/unikraft"
    }
  }
}

source "unikraft-builder" "example" {
  // Platform of the resulting binaries
  architecture = "x86_64"

  // Platform of the resulting binaries
  platform = "kvm"

  # Path of the resulting binaries
  build_path = "/tmp/test/.unikraft/apps/nginx"
  
  // Path where to pull the sources and build the binaries
  workdir = "/tmp/test"

  // Application to pull and build
  pull_source = "nginx"

  // If to use the default source manifests
  sources_no_default = true

  // If to build with all cores
  fast = true

  // If to keep the kraft.yaml file
  keep_config = true

  // Additional sources to pull
  sources = [ "https://github.com/unikraft/app-nginx.git" ]
}

build {
  sources = [
    "source.unikraft-builder.example"
  ]

  post-processor "unikraft-post-processor" {
    // Type of the packaging method
    type = "initramfs"

    // Source from where to archive
    source = "/tmp/test/.unikraft/apps/nginx/fs0"

    // The resulting archive
    destination = "/tmp/test/.unikraft/apps/nginx/build/initramfs.cpio"
  }

  post-processor "unikraft-post-processor" {
    // Type of the packaging method
    type = "oci"

    // Architecture of the packed binary
    architecture = "x86_64"

    // Platform of the packed binary
    platform = "kvm"

    // Source from where to package
    source = "/tmp/test/.unikraft/apps/nginx"

    // The resulting OCI archive
    destination = "my-registry.io/nginx:latest"
  }
}

