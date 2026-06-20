MODULES     := pygtail pflogsumm check_mailq
INSTALL_DIR := /usr/local/bin
HOST        ?= mx01
VERSION     ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")

.PHONY: all build test lint install clean deps fetch-testdata deb rpm pkg

all: build

build:
	@for mod in $(MODULES); do \
		echo "==> Building $$mod"; \
		$(MAKE) -C $$mod build; \
	done

test:
	@for mod in $(MODULES); do \
		echo "==> Testing $$mod"; \
		$(MAKE) -C $$mod test; \
	done

lint:
	@for mod in $(MODULES); do \
		echo "==> Linting $$mod"; \
		(cd $$mod && golangci-lint run ./...); \
	done

install: build
	@for mod in $(MODULES); do \
		echo "==> Installing $$mod"; \
		$(MAKE) -C $$mod install; \
	done
	@echo "Installed to $(INSTALL_DIR)"

clean:
	@for mod in $(MODULES); do \
		echo "==> Cleaning $$mod"; \
		$(MAKE) -C $$mod clean; \
	done

deps:
	@for mod in $(MODULES); do \
		echo "==> Downloading deps for $$mod"; \
		(cd $$mod && go mod download); \
	done

fetch-testdata:
	@echo "Fetching mail.log from $(HOST)..."
	scp $(HOST):/var/log/mail.log pygtail/testdata/mail.log
	cp pygtail/testdata/mail.log pflogsumm/testdata/mail.log
	@echo "Done. Run 'make test' or 'go test -tags integration ./...' inside each module."

deb: build
	@echo "==> Building .deb package v$(VERSION)"
	@bash scripts/build-package.sh deb $(VERSION)

rpm: build
	@echo "==> Building .rpm package v$(VERSION)"
	@bash scripts/build-package.sh rpm $(VERSION)

pkg: deb rpm
	@echo "==> Packages ready in dist/"
