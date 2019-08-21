test:
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck ./...

	go run golang.org/x/lint/golint -set_exit_status
	test -z "$(go run golang.org/x/tools/cmd/goimports -l .)"

	go test -v ./... -race -cover
