all: bin/rce_server bin/rce_client

bin/rce_server:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/rce_server cmd/rce_server/main.go

bin/rce_client:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/rce_client cmd/rce_client/main.go

clean:
	rm -rf bin/*
