
format-proto:
	find . -name "*.proto" -exec clang-format -style=file -i {} \;

gen: format-proto
	cd api && buf generate --timeout 10m -v \
                --path app/

.PHONY: gen

build: gen
	go build -o .mirage/bin/mirage-debug ./cmd
