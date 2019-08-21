test:
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck ./...

	golint -set_exit_status
	test -z "$(goimports -l .)"

	go test -v ./... -race -cover
