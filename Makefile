GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

.PHONY: test binary install clean

cmd/ecspresso/ecspresso: *.go cmd/ecspresso/*.go
	cd cmd/ecspresso && go build -ldflags "-s -w -X main.Version=${GIT_VER} -X main.buildDate=${DATE}" -gcflags="-trimpath=${PWD}"

install: cmd/ecspresso/ecspresso
	install cmd/ecspresso/ecspresso ${GOPATH}/bin

test:
	go test -race .
	go test -race ./cmd/ecspresso

packages:
	cd cmd/ecspresso && gox -os="linux darwin" -arch="amd64" -output "../../pkg/{{.Dir}}-${GIT_VER}-{{.OS}}-{{.Arch}}" -ldflags "-w -s -X main.Version=${GIT_VER} -X main.buildDate=${DATE}"
	cd pkg && find . -name "*${GIT_VER}*" -type f -exec zip {}.zip {} \;

clean:
	rm -f cmd/ecspresso/ecspresso
	rm -f pkg/*

release:
	ghr -u kayac -r ecspresso -n "$(GIT_VER)" $(GIT_VER) pkg/
