# Makefile — fallback for users without Task installed.
# Preferred: install Task (go install github.com/go-task/task/v3/cmd/task@latest)

.PHONY: test lint testcover bench gen clean ci

test:
	go test -race -v ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l -s .)" || (echo "gofmt needed:" && gofmt -l -s . && exit 1)

testcover:
	mkdir -p coverage
	go test -race -coverprofile=coverage/tests.out ./...
	go tool cover -html=coverage/tests.out -o coverage/coverage.html

bench:
	go test -bench=. -benchmem ./...

gen:
	mkdir -p mocks
	mockgen -source=datastore.go -destination=mocks/datastore_mock.go -package=mocks

clean:
	rm -rf coverage/ mocks/ tmp/

ci: lint test testcover
