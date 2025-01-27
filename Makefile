
compile:
	GOOS=linux GOARCH=amd64 go build -o yetis main.go && chmod +x yetis && mv yetis build/

test:
	make test_skip_iptables
	make test_only_iptables

test_skip_iptables:
	go test ./...

test_only_iptables:
	docker run --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp -v /var/run/docker.sock:/var/run/docker.sock glossd/goyetis:0.3-amd\
 sh -c "export TEST_IPTABLES=true; go test -v ./itests/iptables_test.go; go test -v ./proxy/..."

test_one_iptables:
	docker run --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp -v /var/run/docker.sock:/var/run/docker.sock glossd/goyetis:0.3-amd\
 sh -c "export TEST_IPTABLES=true; go test -v ./... -run $(run)"

docker-sh:
	docker run -it --privileged --rm -v $$PWD:/usr/src/myapp -w /usr/src/myapp glossd/goyetis:0.3.1 bash

compile-m1:
	GOOS=darwin GOARCH=arm64 go build -o yetis-m1 main.go && chmod +x yetis-m1 && mv yetis-m1 /usr/local/go/bin/yetis