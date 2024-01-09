# Copyright (c) Unikraft GmbH
# SPDX-License-Identifier: MPL-2.0

# For full specification on the configuration of this file visit:
# https://github.com/hashicorp/integration-template#metadata-configuration
integration {
  name = "Unikraft"
  description = "The Unikraft plugin uses kraftkit to create and package runnable Unikraft unikernel images."
  identifier = "packer/unikraft/unikraft"
  license {
    type = "MPL-2.0"
    url = "https://github.com/unikraft/packer-plugin-unikraft/blob/main/LICENSE"
  }
  component {
    type = "builder"
    name = "Unikraft Kraftkit Building"
    slug = "unikraft"
  }
  component {
    type = "post-processor"
    name = "Unikraft Kraftkit Packaging"
    slug = "unikraft"
  }
}
