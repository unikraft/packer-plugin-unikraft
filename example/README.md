## The Example Folder

This folder contains a fully working example of the plugin usage.
The example defines a `required_plugins` block.
This folder contains a fully working example of the plugin usage.
A GitHub Action runs `packer init`, `packer validate`, and `packer build` to test the plugin with the latest version available of Packer,

The folder can contain multiple HCL2 compatible files.
The action will execute Packer at this folder level running `packer init -upgrade .` and `packer build .`.
