FROM --platform=$BUILDPLATFORM golang:latest AS build

ARG git_tag
RUN git clone -b ${git_tag:?"--build-arg=git_tag=X is required"} https://github.com/gopatchy/bkl.git /repo

ARG TARGETOS TARGETARCH
ENV GOOS=$TARGETOS GOARCH=$TARGETARCH
ENV CGO_ENABLED=0 GOAMD64=v3

RUN mkdir /build && cd /repo && go build -tags bkl-$(git describe --abbrev=0 --tags) -trimpath -ldflags=-extldflags=-static -o /build ./...

FROM scratch AS dist

COPY --from=build /build /bin

LABEL org.opencontainers.image.source https://github.com/gopatchy/bkl
