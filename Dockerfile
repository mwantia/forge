FROM golang:1.25-alpine AS gobuild

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build
ADD . /build

RUN go get -d -v ./...
RUN go run ./tools/plugins -manifest plugins.yaml -out cmd/forge

RUN CGO_ENABLED=0 GOOS=linux go build -tags all -a -ldflags '-s -w -extldflags "-static"' -o ./forge ./cmd/forge/main.go

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