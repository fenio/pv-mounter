export GO111MODULE=on

.PHONY: test
test:
	go test ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: bin
bin: fmt vet
	go build -o bin/pv-mounter github.com/fenio/pv-mounter/cmd/plugin

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet ./pkg/... ./cmd/...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: vuln
vuln:
	govulncheck ./...

.PHONY: docs
docs:
	go run cmd/plugin/docs/docs.go
