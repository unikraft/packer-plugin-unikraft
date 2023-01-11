packer {
  required_plugins {
    unikraft = {
      version = ">=v0.1.0"
      source  = "github.com/unikraft-io/unikraft"
    }
  }
}

source "unikraft-builder" "example1" {
  architecture = "x86_64"
  platform = "kvm"
  build_path = "/tmp/test1/.unikraft/apps/helloworld"
  workdir = "/tmp/test1"
  pull_source = "helloworld"
  source_source = "https://github.com/unikraft/app-helloworld"
}


build {
  sources = [
    "source.unikraft-builder.example1",
  ]

  source "source.unikraft-builder.example2" {
    architecture = "x86_64"
    platform = "kvm"
    build_path = "/tmp/test2/.unikraft/apps/nginx"
    workdir = "/tmp/test2"
    pull_source = "nginx"
    source_source = "https://github.com/unikraft/app-nginx"

  }

  post-processor "unikraft-post-processor" {
    type   = "cpio"
    source = "/tmp/test/.unikraft/apps/nginx/fs0"
    destination = "/tmp/test/.unikraft/apps/nginx/build/initramfs.cpio"
  }
}
