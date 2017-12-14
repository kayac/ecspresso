GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)

.PHONY: test get-deps get-deps-amd64 binary install clean

cmd/ecspresso/ecspresso: *.go cmd/ecspresso/*.go
	cd cmd/ecspresso && go build -ldflags "-s -w -X main.version=${GIT_VER} -X main.buildDate=${DATE}" -gcflags="-trimpath=${PWD}"

install: cmd/ecspresso/ecspresso
	install cmd/ecspresso/ecspresso ${GOPATH}/bin

test:
	go test -race .
	go test -race ./cmd/ecspresso

get-dep-amd64:
	wget -O ${GOPATH}/bin/dep https://github.com/golang/dep/releases/download/v0.3.2/dep-linux-amd64
	chmod +x ${GOPATH}/bin/dep

get-deps:
	dep ensure

packages:
	cd cmd/ecspresso && gox -os="linux darwin" -arch="amd64" -output "../../pkg/{{.Dir}}-${GIT_VER}-{{.OS}}-{{.Arch}}" -ldflags "-w -s -X main.version=${GIT_VER} -X main.buildDate=${DATE}"
	cd pkg && find . -name "*${GIT_VER}*" -type f -exec zip {}.zip {} \;

clean:
	rm -f cmd/ecspresso/ecspresso
	rm -f pkg/*
