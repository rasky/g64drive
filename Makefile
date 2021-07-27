.PHONY: release linux

all:
	@echo "Usage:"
	@echo "•  make release    - build release binary on Mac"
	@echo "•  make linux      - build release binary on Linux through Docker"

release:
	go build -ldflags '-s -w' -o g64drive-mac.binary
	upx --quiet --quiet --lzma g64drive-mac.binary

linux:
	docker build -t g64drive_linux .
	docker run --rm --entrypoint cat g64drive_linux /src/g64drive_linux >g64drive-linux64.binary
	chmod +x g64drive-linux64.binary
	upx --quiet --quiet --lzma g64drive-linux64.binary
