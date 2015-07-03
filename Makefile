
.PHONY: _etcd-amb container

IMAGE_NAME=crewjam/etcd-amb

all: container

container: etcd-amb
	docker build -t $(IMAGE_NAME) .

etcd-amb: etcd-amb.go
	docker run -v $(PWD):/go/src/github.com/crewjam/etcd-amb golang \
		make -C /go/src/github.com/crewjam/etcd-amb _etcd-amb

_etcd-amb:
	go get ./...
	CGO_ENABLED=0 go install -a -installsuffix cgo -ldflags '-s' .
	ldd /go/bin/etcd-amb | grep "not a dynamic executable"
	install /go/bin/etcd-amb etcd-amb
	
lint:
	go fmt ./...
	goimports -w *.go
