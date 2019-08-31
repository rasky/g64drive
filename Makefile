.PHONY: all linux

all:
	go build -ldflags '-s -w'
	upx --quiet --quiet --lzma g64drive

linux:
	docker build -t g64drive_linux .
	docker run --rm --entrypoint cat g64drive_linux /src/g64drive_linux >g64drive_linux
	chmod +x g64drive_linux
	upx --quiet --quiet --lzma g64drive_linux
