build:
	@./build.sh
all:
	@./build.sh all
clean:
	rm -rf rr
install: all
	cp rr /usr/local/bin/rr
uninstall:
	rm -f /usr/local/bin/rr
test:
	composer update
	go test -v -race -cover
lint:
	go fmt ./...
	golint ./...