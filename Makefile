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

.PHONY: demos
demos:
	@command -v vhs >/dev/null 2>&1 || { echo >&2 "vhs is required but not installed. Visit https://github.com/charmbracelet/vhs"; exit 1; }
	@echo "Generating demo GIFs..."
	cd demo && vhs mounted.tape
	cd demo && vhs unmounted.tape
	@echo "Demo GIFs generated successfully!"
