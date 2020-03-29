.PHONY: clean
clean:
	rm -rf build/_output

# Build binary
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/redis-operator -ldflags "-X main.commit=${COMMIT} -X main.version=${VERSION}" ./cmd/manager

.PHONY: test
test:
	go test -v -coverprofile=coverage.txt ./...

check-fmt:
	test -z "$(shell gofmt -l .)"

lint:
	OUTPUT="$(shell go list ./...)"; golint -set_exit_status $$OUTPUT

vet:
	VET_OUTPUT="$(shell go list ./...)"; GO111MODULE=on go vet $$VET_OUTPUT

build-image:
	docker build -t quay.io/opstree/redis-operator:latest -f build/Dockerfile .
