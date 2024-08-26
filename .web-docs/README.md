# Packer Plugin Unikraft
The `Unikraft` multi-component plugin can be used with HashiCorp [Packer](https://www.packer.io)
to create custom images. For the full list of available features for this plugin see [docs](docs).

## Installation

### Using pre-built releases

#### Using the `packer init` command

Starting from version 1.7, Packer supports a new `packer init` command allowing
automatic installation of Packer plugins. Read the
[Packer documentation](https://www.packer.io/docs/commands/init) for more information.

To install this plugin, copy and paste this code into your Packer configuration.
Then, run [`packer init`](https://www.packer.io/docs/commands/init).

```hcl
packer {
  required_plugins {
    unikraft = {
      version = ">= 0.2.1"
      source  = "github.com/unikraft/unikraft"
    }
  }
}
```


#### Manual installation

You can find pre-built binary releases of the plugin [here](https://github.com/unikraft/packer-plugin-unikraft/releases).
Once you have downloaded the latest archive corresponding to your target OS,
uncompress it to retrieve the plugin binary file corresponding to your platform.
To install the plugin, please follow the Packer documentation on
[installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).


### From Sources

If you prefer to build the plugin from sources, clone the GitHub repository
locally and run the command `go build` from the root
directory. Upon successful compilation, a `packer-plugin-unikraft` plugin
binary file can be found in the root directory.
To install the compiled plugin, please follow the official Packer documentation
on [installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).

### Components

#### Builders

unikraft - The builder builds Unikraft image using Kraftkit.

#### Post-Processors

unikraft - The post-processor takes build artifacts from the unikraft builder and packages it into an OCI-compatible image.
