.PHONY: build
build:
		go build -o bin/yc ./cmd

.PHONY: clean
clean:
		rm bin/*
