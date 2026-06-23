build:
	go build -o md .

build_mac:
	GOOS=darwin GOARCH=arm64 go build -o md-mac .

clean:
	rm -f mdr md-mac
