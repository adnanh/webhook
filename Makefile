OS = darwin freebsd linux openbsd
ARCHS = 386 arm amd64 arm64

all: build release release-windows

build: deps
	go build

release: clean deps
	@for arch in $(ARCHS);\
	do \
		for os in $(OS);\
		do \
			echo "Building $$os-$$arch"; \
			mkdir -p build/webhook-$$os-$$arch/; \
			GOOS=$$os GOARCH=$$arch go build -o build/webhook-$$os-$$arch/webhook; \
			tar cz -C build -f build/webhook-$$os-$$arch.tar.gz webhook-$$os-$$arch; \
		done \
	done

release-windows: clean deps
	@for arch in $(ARCHS);\
	do \
		echo "Building windows-$$arch"; \
		mkdir -p build/webhook-windows-$$arch/; \
		GOOS=windows GOARCH=$$arch go build -o build/webhook-windows-$$arch/webhook.exe; \
		tar cz -C build -f build/webhook-windows-$$arch.tar.gz webhook-windows-$$arch; \
	done

test: deps
	go test ./...

deps:
	go get -d -v -t ./...

clean:
	rm -rf build
	rm -f webhook
