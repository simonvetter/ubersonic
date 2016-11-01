GOPATH:=GOPATH=$(shell pwd)/vendor
GOENV:=$(GOPATH)
GOFILES:=$(wildcard src/*.go)

all: clean godeps build-server build-indexer install

clean:
	rm -rf bin/*

godeps:
	$(GOENV) go get github.com/mattn/go-sqlite3

build-server: $(GOFILES)
	$(GOENV) go build -o bin/ubersonic-server $(GOFILES)

build-indexer: src/indexer.cpp
	gcc src/indexer.cpp -std=c++11 -ltag -lstdc++ -lsqlite3 -g -o bin/ubersonic-indexer

install:
	mkdir -p /opt/ubersonic/bin
	cp bin/ubersonic-server bin/ubersonic-indexer /opt/ubersonic/bin/
