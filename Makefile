TEST?=./...
NAME = $(shell basename `pwd`)

all: deps build

deps:
	go get -d -v ./...

updatedeps:
	go get -u -v ./...

build: deps
	@mkdir -p bin/
	go build -o bin/$(NAME)

test: deps
	go test $(TEST) $(TESTARGS) -timeout=30s -parallel=4
	go vet $(TEST)

xcompile: deps test
	@rm -rf build/
	@mkdir -p build
	gox \
		-os="darwin" \
		-os="dragonfly" \
		-os="freebsd" \
		-os="linux" \
		-os="openbsd" \
		-os="solaris" \
		-os="windows" \
		-output="build/{{.Dir}}_{{.OS}}_{{.Arch}}/$(NAME)"

package: xcompile
	$(eval FILES := $(shell ls build))
	@mkdir -p build/tgz
	for f in $(FILES); do \
		(cd $(shell pwd)/build && tar -zcvf tgz/$$f.tar.gz $$f); \
		echo $$f; \
	done

clean:
	@rm -rf build/
	@rm -rf bin/

.PHONY: all deps updatedeps build test xcompile package clean
