binary := "multi"

# build the multi binary into ./bin
build:
    go build -ldflags "-X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/{{binary}} ./cmd/multi

# install multi into GOBIN / GOPATH/bin
install:
    go install ./cmd/multi

# run go vet and tests
check:
    go vet ./...
    go test ./...

# tidy modules
tidy:
    go mod tidy

# build and run against a throwaway brain in /tmp
demo: build
    rm -rf /tmp/multi-demo
    ./bin/{{binary}} init /tmp/multi-demo --name demo --split domain,operations
    ./bin/{{binary}} -b /tmp/multi-demo write --title "First Note" --summary "the very first note in the demo brain" --tags domain --source "manual" --freshness "fresh" --body "Hello brain."
    ./bin/{{binary}} -b /tmp/multi-demo list
    ./bin/{{binary}} -b /tmp/multi-demo lint
