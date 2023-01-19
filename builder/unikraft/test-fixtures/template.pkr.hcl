source "unikraft-builder" "example" {
  architecture = "x86_64"
  platform = "kvm"
  build_path = "/tmp/test/.unikraft/apps/helloworld"
  workdir = "/tmp/test"
  pull_source = "helloworld"
  sources_no_default = false
  sources = [ "https://github.com/unikraft/app-helloworld",
              "https://github.com/unikraft/app-redis"
  ]

}

build {
  sources = [ "source.unikraft-builder.example" ]
}
