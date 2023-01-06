source "kraft-builder" "example" {
  architecture = "x86_64"
  platform = "kvm"
  build_path = "/tmp/test/.unikraft/apps/helloworld"
  workdir = "/tmp/test"
  pull_source = "helloworld"
  source_source = "https://github.com/unikraft/app-helloworld"
}

build {
  sources = [ "source.kraft-builder.example" ]
}
