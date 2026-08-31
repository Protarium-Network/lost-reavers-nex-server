BINARY := lost-reavers

.PHONY: default build run test tidy clean docker

default: build

build:
	go build -o build/$(BINARY) .

run: build
	./build/$(BINARY)

# Most of the database tests need a real Postgres, because every query uses
# Postgres-specific SQL. Without PN_LR_TEST_POSTGRES_URI they skip.
test:
	go vet ./...
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf build

docker:
	docker build -t $(BINARY) .
