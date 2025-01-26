
compile:
	GOOS=linux GOARCH=amd64 go build -o yetis main.go && chmod +x yetis && mv yetis build/

test:
	docker run --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp glossd/goyetis:0.3 go test github.com/glossd/yetis/itests -run TestLivenessRestart

docker-sh:
	docker run -it --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp glossd/goyetis:0.3 bash

compile-m1:
	GOOS=darwin GOARCH=arm64 go build -o yetis-m1 main.go && chmod +x yetis-m1 && mv yetis-m1 /usr/local/go/bin/yetis