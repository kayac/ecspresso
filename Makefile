GIT_VER ?= $(shell git describe --tags | sed -e 's/-/+/')
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

.PHONY: test binary install clean

cmd/ecspresso/ecspresso: *.go cmd/ecspresso/*.go go.* */*.go
	cd cmd/ecspresso && go build -ldflags "-s -w -X main.Version=${GIT_VER} -X main.buildDate=${DATE}" -gcflags="-trimpath=${PWD}"

install: cmd/ecspresso/ecspresso
	install cmd/ecspresso/ecspresso `go env GOPATH`/bin/ecspresso

test:
	go test -race ./...

packages:
	cd cmd/ecspresso && gox -os="linux darwin" -arch="amd64 arm64" -output "../../pkg/{{.Dir}}-${GIT_VER}-{{.OS}}-{{.Arch}}" -ldflags "-w -s -X main.Version=${GIT_VER} -X main.buildDate=${DATE}"
	cd pkg && find . -name "*${GIT_VER}*" -type f -exec zip {}.zip {} \;

clean:
	rm -f cmd/ecspresso/ecspresso
	rm -f pkg/*

release:
	ghr -prerelease -u kayac -r ecspresso -n "$(GIT_VER)" $(GIT_VER) pkg/

ci-test:
	$(MAKE) install
	cd tests/ci && PATH=${GOPATH}/bin:$PATH $(MAKE) test

orb/publish:
	circleci orb validate orb.yml
	circleci orb publish orb.yml $(ORB_NAMESPACE)/ecspresso@dev:latest

orb/promote:
	circleci orb publish promote $(ORB_NAMESPACE)/ecspresso@dev:latest patch
