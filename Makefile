
compile:
	GOOS=linux GOARCH=amd64 go build -o yetis main.go && chmod +x yetis && mv yetis build/

compile-m1:
	GOOS=darwin GOARCH=arm64 go build -o yetis-m1 main.go && chmod +x yetis-m1 && mv yetis-m1 /usr/local/go/bin/yetis