# Makefile — fallback for users without Task installed.
# Preferred: install Task (go install github.com/go-task/task/v3/cmd/task@latest)

.PHONY: test lint fieldalignment-fix fmt testcover bench gen example examples profile-size clean ci

fieldalignment-fix:
	fieldalignment -fix ./...

lint: fieldalignment-fix
	gofmt -l -s -w .
	golangci-lint run

fmt:
	go fmt ./...

test:
	go test -race -v ./...

testcover:
	mkdir -p coverage
	go test -race -coverprofile=coverage/tests.out ./...
	go tool cover -html=coverage/tests.out -o coverage/coverage.html

bench:
	go test -bench=. -benchmem ./...

gen:
	mkdir -p mocks
	mockgen -source=datastore.go -destination=mocks/datastore_mock.go -package=mocks

profile-size:
	@mkdir -p tmp
	@go test -c -o tmp/size-profile.exe .
	@echo "=== Top packages by binary size contribution ==="
	@go tool nm -size tmp/size-profile.exe | \
		awk '{ \
			sym=$$4; n=split(sym, a, "/"); \
			if (sym ~ /^github\.com\//) { pkg=a[1]"/"a[2]"/"a[3]; sub(/\..*/, "", a[3]); pkg=a[1]"/"a[2]"/"a[3] } \
			else if (sym ~ /^go\.opentelemetry\.io\//) { pkg=a[1]"/"a[2] } \
			else if (sym ~ /^google\.golang\.org\//) { pkg=a[1]"/"a[2]"/"a[3] } \
			else if (sym ~ /^golang\.org\//) { pkg=a[1]"/"a[2]"/"a[3] } \
			else if (sym ~ /^gopkg\.in\//) { pkg=a[1]"/"a[2] } \
			else { split(sym, b, "."); pkg=b[1] } \
			size[pkg]+=$$2 \
		} END { for (p in size) printf "%10.2f KB  %s\n", size[p]/1024, p }' | \
		sort -rn | head -30

clean:
	rm -rf coverage/ mocks/ tmp/

ci: lint test testcover
