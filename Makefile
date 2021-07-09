GIT_VER ?= $(shell git describe --tags | sed -e 's/-/+/')
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

.PHONY: test binary install clean

cmd/ecspresso/ecspresso: *.go cmd/ecspresso/*.go go.* */*.go
	cd cmd/ecspresso && go build -ldflags "-s -w -X main.Version=${GIT_VER} -X main.buildDate=${DATE}" -trimpath

install: cmd/ecspresso/ecspresso
	install cmd/ecspresso/ecspresso `go env GOPATH`/bin/ecspresso

test:
	go test -race ./...

packages:
	goreleaser build --skip-validate --rm-dist

clean:
	rm -f cmd/ecspresso/ecspresso
	rm -f dist/*

ci-test:
	$(MAKE) install
	cd tests/ci && PATH=${GOPATH}/bin:$PATH $(MAKE) test

orb/publish:
	circleci orb validate orb.yml
	circleci orb publish orb.yml $(ORB_NAMESPACE)/ecspresso@dev:latest

orb/promote:
	circleci orb publish promote $(ORB_NAMESPACE)/ecspresso@dev:latest patch
