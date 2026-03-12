APP_NAME := zombie
CMD_DIR  := ./cmd/zombie

.PHONY: build run clean test vet fmt install

build:
	go build -o $(APP_NAME) $(CMD_DIR)

run: build
	./$(APP_NAME)

clean:
	rm -f $(APP_NAME)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

install:
	go install $(CMD_DIR)
