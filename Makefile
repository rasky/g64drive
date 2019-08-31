.PHONY: release linux

all:
	@echo "Usage:"
	@echo "•  make release    - build release binary on Mac"
	@echo "•  make linux      - build release binary on Linux through Docker"

release:
	go build -ldflags '-s -w'
	upx --quiet --quiet --lzma g64drive

linux:
	docker build -t g64drive_linux .
	docker run --rm --entrypoint cat g64drive_linux /src/g64drive_linux >g64drive_linux
	chmod +x g64drive_linux
	upx --quiet --quiet --lzma g64drive_linux
