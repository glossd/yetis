
compile:
	GOOS=linux GOARCH=amd64 go build -o yetis main.go && chmod +x yetis && mv yetis build/

test:
	make test_basic
	make test_iptables
	make test_root

test_basic:
	go test ./...

test_iptables:
	docker run --platform linux/amd64 --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp -v /var/run/docker.sock:/var/run/docker.sock glossd/goyetis:0.4-amd\
 sh -c "export TEST_IPTABLES=true; go test -v ./itests/iptables_test.go; go test -v ./proxy/..."

test_one_iptables:
	docker run --platform linux/amd64 --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp -v /var/run/docker.sock:/var/run/docker.sock glossd/goyetis:0.4-amd\
 sh -c "export TEST_IPTABLES=true; go test -v ./... -run $(run)"

test_root:
	docker run --platform linux/amd64 --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp -v /var/run/docker.sock:/var/run/docker.sock glossd/goyetis:0.4-amd\
 sh -c "groupadd -g 10001 appgroup; useradd -u 10001 -g appgroup -m -s /bin/bash appuser; go test -v ./itests -run TestRoot"


docker-sh:
	docker run -it --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp glossd/goyetis:0.3.1 bash

docker-build:
	cd .github && docker build --platform linux/amd64 -t glossd/goyetis:0.4-amd .

compile-m1:
	GOOS=darwin GOARCH=arm64 go build -o yetis-m1 main.go && chmod +x yetis-m1 && mv yetis-m1 /usr/local/go/bin/yetis