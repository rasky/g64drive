.PHONY: release linux

all:
	@echo "Usage:"
	@echo "•  make release        - build release binary on macOS"
	@echo "•  make cross-linux    - build release binary on Linux through Docker"
	@echo "•  make cross-windows  - build release binary on Windows"

release:
	go build -ldflags '-s -w' -o g64drive-mac.binary
	upx --quiet --quiet --lzma g64drive-mac.binary

cross-linux:
	docker build -t g64drive_linux .
	docker run --rm --entrypoint cat g64drive_linux /src/g64drive_linux >g64drive-linux64.binary
	chmod +x g64drive-linux64.binary
	upx --quiet --quiet --lzma g64drive-linux64.binary

cross-windows:
	env GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="x86_64-w64-mingw32-gcc" go build -ldflags '-s -w'
	upx --quiet --quiet --lzma g64drive.exe
