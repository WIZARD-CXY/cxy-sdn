all: build test

build:
	go build -v
	#docker build -t wizardcxy/cxy-sdn .

test:
	#cd server && go test -covermode=count -test.short -coverprofile=coverage.out -v
	cd util && go test -covermode=count -test.short -coverprofile=coverage.out -v
	cd agent && go test -covermode=count -test.short -coverprofile=coverage.out -v
