# Unikraft Plugin

<!--
  Include a short overview about the plugin.

  This document is a great location for creating a table of contents for each
  of the components the plugin may provide. This document should load automatically
  when navigating to the docs directory for a plugin.

-->

## Installation

### Using pre-built releases

#### Using the `packer init` command

Starting from version 1.7, Packer supports a new `packer init` command allowing automatic installation of Packer plugins.\
Read the [Packer documentation](https://www.packer.io/docs/commands/init) for more information.

To install this plugin, copy and paste this code into your Packer configuration.
Then, run [`packer init`](https://www.packer.io/docs/commands/init).

```hcl
packer {
  required_plugins {
    unikraft = {
      version = ">= 0.1.0"
      source  = "github.com/unikraft-io/unikraft"
    }
  }
}
```

#### Manual installation

You can find pre-built binary releases of the plugin [here](https://github.com/unikraft-io/packer-plugin-unikraft/releases).
Once you have downloaded the latest archive corresponding to your target OS,
uncompress it to retrieve the plugin binary file corresponding to your platform.
To install the plugin, please follow the Packer documentation on
[installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).


#### From Source

If you prefer to build the plugin from its source code, clone the GitHub
repository locally and run the command `go build .` from the root
directory. Upon successful compilation, a `packer-plugin-unikraft` plugin
binary file can be found in the root directory.
To install the compiled plugin, please follow the official Packer documentation
on [installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).


## Plugin Contents

The Unikraft plugin contains the following components, which are documented in part.
Some of them are stubbed and/or missing as they do not make sense in the context of Unikraft.

### Builders

- [builder](/docs/builders/builder-name.mdx) - The Unikraft builder is used to create endless Packer
  plugins using a consistent plugin structure.

### Provisioners

- [provisioner](/docs/provisioners/provisioner-name.mdx) - The Unikraft provisioner is used to provision
  Packer builds.

### Post-processors

- [post-processor](/docs/post-processors/postprocessor-name.mdx) - The Unikraft post-processor is used to
  export Unikraft builds.

### Data Sources

- [data source](/docs/datasources/datasource-name.mdx) - The Unikraft data source is used to
  export unikraft data.

