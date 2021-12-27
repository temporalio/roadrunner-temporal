# Run tests temporal server
start-docker-compose:
	docker-compose -f tests/docker-compose.yml up -d

stop-docker-compose:
	docker-compose -f tests/docker-compose.yml down

# Installs all needed composer dependencies
install-dependencies:
	composer --working-dir=./tests/php_test_files install

# Run all tests and code-style checks
test:
	docker-compose -f tests/env/docker-compose.yaml up -d --remove-orphans
	sleep 20
	go test -v -race -cover -tags=debug -failfast  ./tests
	go test -v -race -cover -tags=debug -failfast  ./internal/data_converter
	go test -v -race -cover -tags=debug -failfast  ./workflow/canceller
	go test -v -race -cover -tags=debug -failfast  ./workflow/queue
	docker compose -f tests/env/docker-compose.yaml down

test_coverage:
	docker-compose -f tests/env/docker-compose.yaml up -d --remove-orphans
	rm -rf coverage-ci
	mkdir ./coverage-ci
	go test -v -race -cover -tags=debug -failfast -coverpkg=./... -coverprofile=./coverage-ci/temporal.out -covermode=atomic ./tests
	go test -v -race -cover -tags=debug -failfast -coverpkg=./... -coverprofile=./coverage-ci/temporal_protocol.out -covermode=atomic ./internal/data_converter
	go test -v -race -cover -tags=debug -failfast -coverpkg=./... -coverprofile=./coverage-ci/temporal_workflow.out -covermode=atomic ./workflow
	go test -v -race -cover -tags=debug -failfast -coverpkg=./... -coverprofile=./coverage-ci/canceller.out -covermode=atomic ./workflow/canceller
	go test -v -race -cover -tags=debug -failfast -coverpkg=./... -coverprofile=./coverage-ci/queue.out -covermode=atomic ./workflow/queue
	echo 'mode: atomic' > ./coverage-ci/summary.txt
	tail -q -n +2 ./coverage-ci/*.out >> ./coverage-ci/summary.txt
	docker-compose -f tests/env/docker-compose.yaml down

generate-proto:
	protoc -I./proto/api  -I./proto  --go_out=proto/protocol/v1 ./proto/protocol/v1/protocol.proto