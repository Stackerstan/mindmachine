BINARY_NAME=mindmachine

build:
	go mod tidy
	go build -o ${BINARY_NAME} cmd/mindmachine/*.go

install:
	go mod tidy
	go build -o ${BINARY_NAME} cmd/mindmachine/*.go
	cp ${BINARY_NAME} ~/go/bin/

clean:
	rm ${BINARY_NAME}

run:
	go mod tidy
	go run cmd/mindmachine/*.go

debug:
	go mod tidy
	~/go/bin/dlv debug cmd/mindmachine/*.go

debugreset:
	cp -R ~/mindmachine/data/nostrelay ~/mindmachine/
	rm -rf ~/mindmachine/data
	mkdir ~/mindmachine/data
	cp -R ~/mindmachine/nostrelay ~/mindmachine/data/
	rm -rf ~/mindmachine/nostrelay
	go mod tidy
	~/go/bin/dlv debug cmd/mindmachine/*.go

reset:
	cp -R ~/mindmachine/data/nostrelay ~/mindmachine/
	rm -rf ~/mindmachine/data
	mkdir ~/mindmachine/data
	cp -R ~/mindmachine/nostrelay ~/mindmachine/data/
	rm -rf ~/mindmachine/nostrelay
	go mod tidy
	go run cmd/mindmachine/*.go

clearconfig:
	cp ~/mindmachine/config.yaml ~/mindmachine/config.yaml.old
	rm ~/mindmachine/config.yaml