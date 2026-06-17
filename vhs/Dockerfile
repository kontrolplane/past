# Records the README demo entirely inside a container, so generating it never
# puts a gpg key, password store, agent, or toolchain on the host. Build context
# is the repo root: `docker build -f vhs/Dockerfile .` (wired up in `make vhs`).
FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /past .

FROM ghcr.io/charmbracelet/vhs:latest
RUN apt-get update \
	&& apt-get install -y --no-install-recommends pass gnupg git \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=build /past /usr/local/bin/past

COPY vhs/cassette.tape /vhs/cassette.tape

COPY vhs/secrets/ /vhs/secrets/

# Seed a throwaway gpg key + pass store and load the demo secrets, so the
# recording drives a populated `past`. Empty passphrase: this image is built
# fresh per recording and never leaves the container.
ENV GNUPGHOME=/root/.gnupg PASSWORD_STORE_DIR=/root/.password-store
RUN gpg --batch --pinentry-mode loopback --passphrase '' \
		--quick-generate-key 'past demo <demo@past.local>' default default never \
	&& pass init demo@past.local \
	&& pass git init \
	&& past import /vhs/secrets/secrets.csv

WORKDIR /vhs

ENTRYPOINT ["vhs", "cassette.tape"]
