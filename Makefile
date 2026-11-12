.PHONY: build run test check smoke native-test package
build:
	go build -o bin/xiterm ./cmd/xiterm
run:
	go run ./cmd/xiterm
test:
	go test ./assets ./internal/config ./internal/terminal ./internal/workspace ./internal/app github.com/charmbracelet/x/vt github.com/onix-engine/onix/font github.com/onix-engine/onix/render
check:
	go vet ./assets ./cmd/xiterm ./internal/config ./internal/terminal ./internal/workspace ./internal/app github.com/charmbracelet/x/vt
	# Native FFI handles use uintptr; exclude the pointer-cast analyzer there.
	go vet -unsafeptr=false github.com/onix-engine/onix/font github.com/onix-engine/onix/render
smoke: build
	xvfb-run -a ./bin/xiterm -smoke-duration=3s

native-test: build
	xvfb-run -a -s '-screen 0 1280x860x24' python3 scripts/native_smoke.py

# Example: make package TAG=v0.1.0
package:
	python3 scripts/package.py --tag "$(TAG)"
