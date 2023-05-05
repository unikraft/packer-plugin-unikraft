NAME=unikraft
BINARY=packer-plugin-${NAME}

COUNT?=1
TEST?=$(shell go list ./...)
HASHICORP_PACKER_PLUGIN_SDK_VERSION?=$(shell go list -m github.com/hashicorp/packer-plugin-sdk | cut -d " " -f2)

CMAKE       ?= cmake
WORKDIR     ?= $(CURDIR)
VENDORDIR   ?= $(WORKDIR)/third_party
GO_VERSION  ?= 1.20

.PHONY: dev

build: git2go
	@go build -o ${BINARY}

dev: build
	@mkdir -p ~/.packer.d/plugins/
	@mv ${BINARY} ~/.packer.d/plugins/${BINARY}

test:
	@go test -race -count $(COUNT) $(TEST) -timeout=3m

install-packer-sdc: ## Install packer sofware development command
	@go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@v0.4.0

ci-release-docs: install-packer-sdc
	@packer-sdc renderdocs -src docs -partials docs-partials/ -dst docs/
	@/bin/sh -c "[ -d docs ] && zip -r docs.zip docs/"

plugin-check: install-packer-sdc build
	@packer-sdc plugin-check ${BINARY}

testacc: dev
	@PACKER_ACC=1 go test -count $(COUNT) -v $(TEST) -timeout=120m

generate: install-packer-sdc
	@go generate ./...

.PHONY: git2go
git2go: $(VENDORDIR)/libgit2/git2go/static-build/install/lib/pkgconfig/libgit2.pc
	@go install -tags static github.com/libgit2/git2go/v31/...

$(VENDORDIR)/libgit2/git2go/static-build/install/lib/pkgconfig/libgit2.pc: $(VENDORDIR)/libgit2/git2go/vendor/libgit2
	@mkdir -p $(VENDORDIR)/libgit2/git2go/static-build/build
	@mkdir -p $(VENDORDIR)/libgit2/git2go/static-build/install
	(cd $(VENDORDIR)/libgit2/git2go/static-build/build && $(CMAKE) \
		-DTHREADSAFE=ON \
		-DBUILD_CLAR=OFF \
		-DBUILD_SHARED_LIBS=OFF \
		-DREGEX_BACKEND=builtin \
		-DUSE_BUNDLED_ZLIB=ON \
		-DUSE_HTTPS=ON \
		-DUSE_SSH=ON \
		-DCMAKE_C_FLAGS=-fPIC \
		-DCMAKE_BUILD_TYPE="RelWithDebInfo" \
		-DCMAKE_INSTALL_PREFIX=$(VENDORDIR)/libgit2/git2go/static-build/install \
		-DCMAKE_INSTALL_LIBDIR="lib" \
		-DDEPRECATE_HARD="${BUILD_DEPRECATE_HARD}" \
		$(VENDORDIR)/libgit2/git2go/vendor/libgit2)
	$(MAKE) -C $(VENDORDIR)/libgit2/git2go/static-build/build install

$(VENDORDIR)/libgit2/git2go/vendor/libgit2: $(VENDORDIR)/libgit2/git2go
	@git -C $(VENDORDIR)/libgit2/git2go submodule update --init --recursive

$(VENDORDIR)/libgit2/git2go:
	@git clone --branch v31.7.9 --recurse-submodules https://github.com/libgit2/git2go.git $@
