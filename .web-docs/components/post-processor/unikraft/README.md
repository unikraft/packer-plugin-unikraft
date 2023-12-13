This allows you to export the resulting binary in different formats (e.g. OCI).

**Required**

- `source` (string) - The source directory to create the archive from. The source directory must contain a `kraft.yaml` file.
- `destination` (string) - The resulting package file. The `destination` must be a valid OCI image name.
- `architecture` (string) - The architecture of the packaged image.
- `platform` (string) - The platform of the packaged image.

**Optional**

- `target` (string) - The target of the packaged image.
- `push` (bool) - If to push the resulting image to the registry.
- `rootfs` (string) - The path to the rootfs of the packaged image.
- `log_level` (string) - The log level of the packaged image. Can be `debug`, `info`, `warn`, `error`, `fatal`, `panic`. Default: `info`.

### Example Usage

```hcl
post-processor "kraft-pkg" {
  source = "/tmp/example/.unikraft/apps/helloworld"
  destination = "my-registry.io/helloworld:latest"
  architecture = "x86_64"
  platform = "qemu"
  rootfs = "/tmp/example/.unikraft/apps/helloworld/rootfs"
  push = true
  log_level = "info"
}
```
