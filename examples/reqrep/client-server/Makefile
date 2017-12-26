# Build file for simple req/rep client server
TARGETS := server client

.PHONY: all clean

all: $(TARGETS)
	@./server --help || true
	@./client --help || true

server: aux.go server.go
	go build -race -o $@ $^

client: aux.go client.go
	go build -tags client -race -o $@ $^

clean:
	$(RM) $(TARGETS) *~
