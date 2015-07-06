all: build test

build:
	go build -v
	docker build -t 10.10.103.215:5000/cxy-sdn .
push:
	docker push 10.10.103.215:5000/cxy-sdn
test:
	cd server && go test -covermode=count -test.short -coverprofile=coverage.out -v
	cd util && go test -covermode=count -test.short -coverprofile=coverage.out -v
	cd agent && go test -covermode=count -test.short -coverprofile=coverage.out -v
