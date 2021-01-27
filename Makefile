# Run tests temporal server
start-docker-compose:
	docker-compose -f tests/docker-compose.yml up -d

stop-docker-compose:
	docker-compose -f tests/docker-compose.yml down

# Installs all needed composer dependencies
install-dependencies:
	go get -u golang.org/x/lint/golint
	composer --working-dir=./test/ install

# Run integration tests
test-feature:
	go test -v -race -tags=debug ./tests

test-unit:
	go test -v -race -tags=debug ./protocol/
	go test -v -race -tags=debug ./workflow

# Run all tests and code-style checks
test: test-unit test-feature