BINARY := past
PREFIX ?= /usr/local

.PHONY: build run test fmt vet tidy install clean vhs

build:
	go build -o $(BINARY) .

run:
	go run .

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

install: build
	install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)

clean:
	rm -f $(BINARY)

# regenerate the README gif and pages inside a container, so nothing but the
# output (written to ./assets) ever touches the host — no gpg key, store, or agent.
vhs:
	docker build -t past-vhs -f vhs/Dockerfile .
	mkdir -p assets/pages/store assets/pages/entry
	docker run --rm -v "$(CURDIR)/assets:/out" past-vhs
