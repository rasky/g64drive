# Base build image

FROM golang:1.16-alpine AS build_base

# Install some dependencies needed to build the project
RUN apk add git ca-certificates gcc make libc-dev libftdi1-dev libftdi1-static

# Compile libusb from source code because Alpine does not ship a package with
# libusb as static library.
RUN apk add eudev-dev linux-headers
RUN mkdir /tmp/libusb && \
	cd /tmp/libusb && \
	wget -q https://github.com/libusb/libusb/releases/download/v1.0.24/libusb-1.0.24.tar.bz2 && \
	tar xvf libusb-1.0.24.tar.bz2 && \
	cd libusb-1.0.24 && \
	./configure --disable-dependency-tracking && \
	make && \
	make install && \
	cd / && \
	rm -rf /tmp/libusb
ENV PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig

# We want to populate the module cache based on the go.{mod,sum} files. 
WORKDIR /src
COPY go.mod .
COPY go.sum .

# Download all required dependencies. We do this separately from go build
# so that we don't have to do it any time our source code changes.
RUN go mod download

# Here we copy the rest of the source code
COPY . .
# And compile the project
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -v -a -tags 'netgo osusergo' -ldflags '-s -w -extldflags "-static -ludev"' -o g64drive_linux
