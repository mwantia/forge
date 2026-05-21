FROM golang:1.25-alpine AS gobuild

ARG TARGETOS
ARG TARGETARCH
ARG VARIANT=none

WORKDIR /build
ADD . /build

RUN go get -d -v ./...

RUN if [ "$VARIANT" != "none" ]; then \
      go run ./tools/plugins -manifest plugins.yaml -out cmd/forge; \
    fi

RUN if [ "$VARIANT" = "none" ]; then \
      CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w -extldflags "-static"' -trimpath -o ./forge ./cmd/forge/main.go; \
    else \
      CGO_ENABLED=0 GOOS=linux go build -tags all -a -ldflags '-s -w -extldflags "-static"' -trimpath -o ./forge ./cmd/forge/main.go; \
    fi

RUN chmod +x ./forge

FROM alpine:3.21.3

ARG TARGETOS
ARG TARGETARCH
# Install required packages and manually update the local certificates
RUN apk add --no-cache tzdata bash ca-certificates && update-ca-certificates
# Copy executable from build
COPY --from=gobuild /build/forge /app/forge
# Expose port 9280 and 9500 by default
EXPOSE 9280/tcp
EXPOSE 9500/tcp
# Set entrypoint to run executable
WORKDIR /app
ENTRYPOINT [ "/app/forge" ]
CMD [ "agent", "--log-level DEBUG" ]