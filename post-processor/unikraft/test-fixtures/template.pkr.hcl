source "unikraft-builder" "example" {
  architecture = "x86_64"
  platform = "kvm"
  build_path = "/tmp/test/.unikraft/apps/helloworld"
  workdir = "/tmp/test"
  pull_source = "helloworld"
  sources = [ "https://github.com/unikraft/app-helloworld" ]
}

build {
  sources = [
    "source.unikraft-builder.example"
  ]

  post-processor "unikraft-post-processor" {
    type   = "cpio"
    source = "/tmp/test/.unikraft/apps/helloworld/fs0"
    destination = "/tmp/test/.unikraft/apps/helloworld/build/initramfs.cpio"
  }
}
