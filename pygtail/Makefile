BINARY    := pygtail
DIST_DIR  := dist
INSTALL_DIR := /opt/zabbix_postfix

.PHONY: build test install clean

build: $(DIST_DIR)/$(BINARY)

$(DIST_DIR)/$(BINARY):
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY) .
	upx --best $(DIST_DIR)/$(BINARY)

test:
	go test ./...

install: build
	install -m 0755 $(DIST_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)

clean:
	rm -rf $(DIST_DIR)
