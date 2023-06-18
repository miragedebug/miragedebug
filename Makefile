
format-proto:
	find . -name "*.proto" -exec clang-format -style=file -i {} \;

gen-d2:
	bash ./scripts/gen-d2.sh

.PHONY: gen-d2

gen: format-proto gen-d2
	cd api && buf generate --timeout 10m -v \
                --path app/

.PHONY: gen

install: gen
	go install ./cmd/mirage-debug
