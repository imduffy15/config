test:
	go get golang.org/x/tools/cmd/goimports
	go get -u golang.org/x/lint/golint

	go vet ./...

	golint -set_exit_status
	test -z "$(goimports -l .)"

	go test -v ./... -race -cover

	go mod tidy
