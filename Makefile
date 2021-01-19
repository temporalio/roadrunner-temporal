# Run test temporal server
start-docker-compose:
	docker-compose -f test/docker-compose.yml up -d

stop-docker-compose:
	docker-compose -f test/docker-compose.yml down

# Installs all needed composer dependencies
install-dependencies:
	go get -u golang.org/x/lint/golint
	composer --working-dir=./test/ install

test-cs:
	golint ./protocol/
	golint ./plugins/activity/
	golint ./plugins/temporal/
	golint ./plugins/workflow/

# Run integration tests
test-feature:
	go test -race -v -count=1 ./test/

test-unit:
	go test -race -v -count=1 ./protocol/
	go test -race -v -count=1 ./plugins/workflow/

# Run all tests and code-style checks
test: test-cs test-unit test-feature