build-local:
	go build -o ./out/mb-tool .

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o ./out/mb-tool_linux_amd64 .

build-linux-arm64:
	rm -f ./out/mb-tool_linux_arm64
	GOOS=linux GOARCH=arm64 go build -o ./out/mb-tool_linux_arm64 .
	upx -9 ./out/mb-tool_linux_arm64

build-linux-armv7:
	GOOS=linux GOARCH=arm GOARM=7 go build -o ./out/mb-tool_linux_armv7 .
	upx -9 ./out/mb-tool_linux_armv7


upload-linux-arm64: build-linux-arm64
	scp -P 6041 ./out/mb-tool_linux_arm64 wudun@aiwatt.net:/home/wudun/file_server/data/tools/mb-tool_linux_arm64
	md5 out/mb-tool_linux_arm64

upload-linux-armv7: build-linux-armv7
	scp -P 6041 ./out/mb-tool_linux_armv7 wudun@aiwatt.net:/home/wudun/file_server/data/tools/mb-tool_linux_armv7
	md5 out/mb-tool_linux_armv7

upload-ali-arm64: build-linux-arm64
	scp ./out/mb-tool_linux_arm64 work@8.142.40.30:/home/www-data/tools/mb-tool_linux_arm64
	md5 ./out/mb-tool_linux_arm64
