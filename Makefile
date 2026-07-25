.PHONY: build web test lint run dev clean fmt vet

BUILD_FLAGS ?= -buildvcs=false
PORT ?= 8082

# web builds CSS/Tailwind assets via npm. Only needed when modifying web/src.
# Not a dependency of build — Go binary embeds whatever is currently in web/static.
web:
	cd web && npm run build

build:
	go build $(BUILD_FLAGS) -o mikvoc ./cmd/mikvoc

run:
	go run $(BUILD_FLAGS) ./cmd/mikvoc/main.go --port $(PORT)

dev: run

test:
	go test $(BUILD_FLAGS) ./...

test-race:
	go test $(BUILD_FLAGS) -race ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

lint: vet
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

clean:
	rm -f mikvoc

all: build test lint
