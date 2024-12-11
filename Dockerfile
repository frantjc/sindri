ARG tool=valheimw

FROM golang:1.23 AS build
WORKDIR $GOPATH/github.com/frantjc/sindri
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG tool=valheimw
ENV CGO_ENABLED 0
RUN go build -o /$tool ./cmd/$tool

FROM debian:stable-slim AS valheimw
RUN apt-get update -y \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        lib32gcc-s1 \
        libatomic1 \
        libpulse-dev \
        libpulse0 \
    && rm -rf /var/lib/apt/lists/*
RUN groupadd -r valheimw
RUN useradd -r -g valheimw -m -d /valheimw -s /bin/bash valheimw
USER valheimw
ENTRYPOINT ["valheimw"]
COPY --from=build /valheimw /usr/local/bin

FROM scratch AS boil
COPY --from=build /boil /
ENTRYPOINT ["/boil"]

FROM scratch AS mist
COPY --from=build /mist /
ENTRYPOINT ["/mist"]

FROM scratch AS sindri
COPY --from=build /sindri /sindri
ENTRYPOINT ["/sindri"]

FROM $tool
