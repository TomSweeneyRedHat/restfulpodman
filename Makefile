# Stolen from LibPod v1.0.0 and somewhat manipulated
GO ?= go
DESTDIR ?= /
EPOCH_TEST_COMMIT ?= 4406e1cfeed18fe89c0ad4e20a3c3b2f4b9ffcae
HEAD ?= HEAD
CHANGELOG_BASE ?= HEAD~
CHANGELOG_TARGET ?= HEAD
#PROJECT := github.com/containers/libpod
PROJECT := github.com/TomSweeneyRedHat/restfulpodman
GIT_BASE_BRANCH ?= origin/master
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN ?= $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
PREFIX ?= ${DESTDIR}/usr/local
BINDIR ?= ${PREFIX}/bin
LIBEXECDIR ?= ${PREFIX}/libexec
MANDIR ?= ${PREFIX}/share/man
SHAREDIR_CONTAINERS ?= ${PREFIX}/share/containers
ETCDIR ?= ${DESTDIR}/etc
TMPFILESDIR ?= ${PREFIX}/lib/tmpfiles.d
BUILDTAGS ?= seccomp $(shell hack/btrfs_tag.sh) $(shell hack/btrfs_installed_tag.sh) $(shell hack/ostree_tag.sh) $(shell hack/selinux_tag.sh) $(shell hack/apparmor_tag.sh) varlink exclude_graphdriver_devicemapper
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)

BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
OCIUMOUNTINSTALLDIR=$(PREFIX)/share/oci-umount/oci-umount.d

SELINUXOPT ?= $(shell test -x /usr/sbin/selinuxenabled && selinuxenabled && echo -Z)
PACKAGES ?= $(shell $(GO) list -tags "${BUILDTAGS}" ./... | grep -v github.com/containers/libpod/vendor | grep -v e2e | grep -v system )

COMMIT_NO ?= $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_COMMIT ?= $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")
BUILD_INFO ?= $(shell date +%s)
ISODATE ?= $(shell date --iso-8601)

# If GOPATH not specified, use one in the local directory
ifeq ($(GOPATH),)
export GOPATH := $(CURDIR)/_output
unexport GOBIN
endif
FIRST_GOPATH := $(firstword $(subst :, ,$(GOPATH)))
GOPKGDIR := $(FIRST_GOPATH)/src/$(PROJECT)
GOPKGBASEDIR ?= $(shell dirname "$(GOPKGDIR)")

GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(FIRST_GOPATH)/bin
endif

GOMD2MAN ?= $(shell command -v go-md2man || echo '$(GOBIN)/go-md2man')

all: binaries docs

default: help

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'install' - Install binaries to system locations"
	@echo " * 'binaries' - Build podman"
	@echo " * 'integration' - Execute integration tests"
	@echo " * 'clean' - Clean artifacts"
	@echo " * 'lint' - Execute the source code linter"
	@echo " * 'gofmt' - Verify the source code gofmt"

.gopathok:
ifeq ("$(wildcard $(GOPKGDIR))","")
	mkdir -p "$(GOPKGBASEDIR)"
	ln -s "$(CURDIR)" "$(GOPKGBASEDIR)"
endif
	touch $@

gofmt:
	find . -name '*.go' ! -path './vendor/*' -exec gofmt -s -w {} \+
	git diff --exit-code

podman: .gopathok $(PODMAN_VARLINK_DEPENDENCIES)
	$(GO) build -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote: .gopathok $(PODMAN_VARLINK_DEPENDENCIES)
	$(GO) build -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS) remoteclient" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote-darwin: .gopathok $(PODMAN_VARLINK_DEPENDENCIES)
	GOOS=darwin $(GO) build -ldflags '$(LDFLAGS_PODMAN)' -tags "remoteclient containers_image_openpgp exclude_graphdriver_devicemapper" -o bin/$@ $(PROJECT)/cmd/podman

clean:
	rm -rf \
		.gopathok \
		_output \
		bin \
		bin/restapi \
		build \
		$(MANPAGES) ||:
	find . -name \*~ -delete
	find . -name \#\* -delete

restapi: .gopathok 
	$(GO) build -i $(LDFLAGS_RESTAPI) -o bin/$@ $(PROJECT)/restapi
#	$(GO) build -i $(LDFLAGS_RESTAPI) -tags "$(BUILDTAGS)" -o bin/$@ $(PROJECT)/restapi

binaries: restapi


MANPAGES_MD ?= $(wildcard docs/*.md pkg/*/docs/*.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)

$(MANPAGES): %: %.md .gopathok
	@sed -e 's/\((podman.*\.md)\)//' -e 's/\[\(podman.*\)\]/\1/' $<  | $(GOMD2MAN) -in /dev/stdin -out $@

docs: $(MANPAGES)

changelog:
	@echo "Creating changelog from $(CHANGELOG_BASE) to $(CHANGELOG_TARGET)"
	$(eval TMPFILE := $(shell mktemp))
	$(shell cat changelog.txt > $(TMPFILE))
	$(shell echo "- Changelog for $(CHANGELOG_TARGET) ($(ISODATE)):" > changelog.txt)
	$(shell git log --no-merges --format="  * %s" $(CHANGELOG_BASE)..$(CHANGELOG_TARGET) >> changelog.txt)
	$(shell echo "" >> changelog.txt)
	$(shell cat $(TMPFILE) >> changelog.txt)
	$(shell rm $(TMPFILE))

install: .gopathok install.bin install.man install.cni install.systemd

install.bin:
	install ${SELINUXOPT} -d -m 755 $(BINDIR)
	install ${SELINUXOPT} -m 755 bin/restapi $(BINDIR)/restapi

install.man: docs
	install ${SELINUXOPT} -d -m 755 $(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(MANDIR)/man5
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES)) -t $(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.5,$(MANPAGES)) -t $(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/links/*1 -t $(MANDIR)/man1

install.cni:
	install ${SELINUXOPT} -d -m 755 ${ETCDIR}/cni/net.d/
	install ${SELINUXOPT} -m 644 cni/87-podman-bridge.conflist ${ETCDIR}/cni/net.d/87-podman-bridge.conflist

uninstall:
	for i in $(filter %.1,$(MANPAGES)); do \
		rm -f $(MANDIR)/man1/$$(basename $${i}); \
	done; \
	for i in $(filter %.5,$(MANPAGES)); do \
		rm -f $(MANDIR)/man5/$$(basename $${i}); \
	done

.PHONY: .gitvalidation
.gitvalidation: .gopathok
	GIT_CHECK_EXCLUDE="./vendor" $(GOBIN)/git-validation -v -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..$(HEAD)

install.tools: .install.gitvalidation .install.gometalinter .install.md2man

.install.gitvalidation: .gopathok
	if [ ! -x "$(GOBIN)/git-validation" ]; then \
		$(GO) get -u github.com/vbatts/git-validation; \
	fi

.install.gometalinter: .gopathok
	if [ ! -x "$(GOBIN)/gometalinter" ]; then \
		$(GO) get -u github.com/alecthomas/gometalinter; \
		cd $(FIRST_GOPATH)/src/github.com/alecthomas/gometalinter; \
		git checkout e8d801238da6f0dfd14078d68f9b53fa50a7eeb5; \
		$(GO) install github.com/alecthomas/gometalinter; \
		$(GOBIN)/gometalinter --install; \
	fi

.install.md2man: .gopathok
	if [ ! -x "$(GOBIN)/go-md2man" ]; then \
		   $(GO) get -u github.com/cpuguy83/go-md2man; \
	fi

validate: gofmt .gitvalidation validate.completions


vendor:
	vndr -whitelist "github.com/varlink/go"

.PHONY: \
	.gopathok \
	binaries \
	clean \
	default \
	docs \
	gofmt \
	help \
	install \
	lint \
	pause \
	uninstall \
	shell \
	changelog \
	validate \
	vendor
