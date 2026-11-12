BIN   := audit
ADDR  ?= 127.0.0.1:8100
FLAGS := -ldflags="-s -w"

.PHONY: all build test vet fmt run smoke clean
all: fmt vet test build

build:
	go build $(FLAGS) -o $(BIN) .

test:
	go test -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

run: build
	./$(BIN) -addr $(ADDR)

# Démarre, interroge /ping, vérifie PONG-V2, arrête. Zéro résidu.
smoke: build
	@./$(BIN) -addr $(ADDR) >/dev/null 2>&1 & pid=$$!; \
	sleep 0.5; \
	code=$$(curl -s -o /dev/null -w '%{http_code}' http://$(ADDR)/ping); \
	body=$$(curl -s http://$(ADDR)/ping); \
	kill -TERM $$pid 2>/dev/null; wait $$pid 2>/dev/null; \
	echo "GET /ping -> $$code [$$body]"; \
	[ "$$code" = "200" ] && [ "$$body" = "PONG-V2" ]

clean:
	rm -f $(BIN)
