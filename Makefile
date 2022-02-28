BUF_VERSION:=1.0.0-rc9

generate:
	docker run -v $$(pwd):/src -w /src --rm bufbuild/buf:$(BUF_VERSION) generate
