go env -w  GO111MODULE=auto
golint *.go
gofmt -w *.go