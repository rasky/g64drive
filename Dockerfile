# Base build image

FROM golang:1.12-alpine AS build_base
# Install some dependencies needed to build the project
RUN apk add git ca-certificates gcc libc-dev libftdi1-dev
WORKDIR /src

# Force the go compiler to use modules 
ENV GO111MODULE=on

# We want to populate the module cache based on the go.{mod,sum} files. 
COPY go.mod .
COPY go.sum .

#This is the ‘magic’ step that will download all the dependencies that are specified in 
# the go.mod and go.sum file.

# Because of how the layer caching system works in Docker, the go mod download 
# command will _ only_ be re-run when the go.mod or go.sum file change 
# (or when we add another docker instruction this line) 
RUN go mod download

# Here we copy the rest of the source code
COPY . .
# And compile the project
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -tags 'netgo osusergo' -ldflags '-s -w -extldflags "-static"' -o g64drive_linux
