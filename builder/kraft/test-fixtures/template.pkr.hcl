source "kraft-builder" "example" {
  architecture = "x86_64"
  platform = "kvm"
  build_path = "/tmp/.unikraft/apps/helloworld"
  workdir = "/tmp/"
  pull_source = "helloworld"
}

build {
  sources = [ "source.kraft-builder.example" ]
}
