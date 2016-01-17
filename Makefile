all: build push

build:
	GO15VENDOREXPERIMENT=1 go build -v
	docker build -t registry.aliyuncs.com/wizardcxy/cxy-sdn .
push:
	docker push registry.aliyuncs.com/wizardcxy/cxy-sdn
test:
	#cd server && go test -covermode=count -test.short -coverprofile=coverage.out -v
	cd util && go test -covermode=count -test.short -coverprofile=coverage.out -v
	cd netAgent && go test -covermode=count -test.short -coverprofile=coverage.out -v
