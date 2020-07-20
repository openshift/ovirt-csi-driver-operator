SHELL :=/bin/bash

all: build
.PHONY: all

TARGET_NAME=csi-driver-operator
IMAGE_REF=quay.io/ovirt/$(TARGET_NAME):latest
GO_TEST_PACKAGES :=./pkg/... ./cmd/...
IMAGE_REGISTRY?=registry.svc.ci.openshift.org

# You can customize go tools depending on the directory layout.
# example:
#GO_BUILD_PACKAGES :=./pkg/...
# You can list all the golang related variables by:
#   $ make -n --print-data-base | grep ^GO

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/deps-gomod.mk \
	targets/openshift/images.mk \
	targets/openshift/bindata.mk \
	targets/openshift/codegen.mk \
)

# Codegen module needs setting these required variables
PACKAGE_BASE := github.com/ovirt/csi-driver-operator
CODEGEN_OUTPUT_PACKAGE :=$(PACKAGE_BASE)/pkg/generated
CODEGEN_API_PACKAGE :=$(PACKAGE_BASE)/pkg/apis
CODEGEN_GROUPS_VERSION :=operator:v1alpha1

ALIAS_GOPATH := /tmp/go/src

alias_this:
	mkdir -p $(ALIAS_GOPATH)/$$(dirname $(PACKAGE_BASE))
	ln -s $$(dirname $$(go env GOMOD)) $(ALIAS_GOPATH)/$(PACKAGE_BASE)

define run-codegen
	"$(SHELL)" \
		"$(CODEGEN_PKG)/generate-groups.sh" \
	"$(CODEGEN_GENERATORS)" \
	"$(CODEGEN_OUTPUT_PACKAGE)" \
	"$(CODEGEN_API_PACKAGE)" \
	"$(CODEGEN_GROUPS_VERSION)" \
    --output-base $(ALIAS_GOPATH) \
    --go-header-file $(CODEGEN_GO_HEADER_FILE) \
    $1
endef

# All the available targets are listed in <this-file>.help
# or you can list it live by using `make help`


# You can list all codegen related variables by:
#   $ make -n --print-data-base | grep ^CODEGEN

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context
# It will generate target "image-$(1)" for builing the image an binding it as a prerequisite to target "images".
$(call build-image,$(TARGET_NAME),$(IMAGE_REF),./Dockerfile,.)

# This will call a macro called "add-bindata" which will generate bindata specific targets based on the parameters:
# $0 - macro name
# $1 - target suffix
# $2 - input dirs
# $3 - prefix
# $4 - pkg
# $5 - output
# It will generate targets {update,verify}-bindata-$(1) logically grouping them in unsuffixed versions of these targets
# and also hooked into {update,verify}-generated for broader integration.
$(call add-bindata,generated,./assets/...,assets,generated,pkg/generated/bindata.go)

# make target aliases
fmt: verify-gofmt
		
vet: verify-govet

.PHONY: vendor
vendor: verify-deps
