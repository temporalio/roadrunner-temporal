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
	go test -v -race -cover -tags=debug -failfast  ./...

test_coverage:
	rm -rf coverage-ci
	mkdir ./coverage-ci
	go test -v -race -cover -tags=debug -failfast -coverpkg=./... -coverprofile=./coverage-ci/temporal.out -covermode=atomic ./...
	echo 'mode: atomic' > ./coverage-ci/summary.txt
	tail -q -n +2 ./coverage-ci/*.out >> ./coverage-ci/summary.txt