SHELL := /bin/sh

all: run

update:
	@go get -u
	@go mod tidy

run:
	@go run cmd/*.go

build:
	@docker build --quiet -t timendus/frugal:latest .
	@mkdir -p dist/docker
	@docker save timendus/frugal:latest | gzip > dist/docker/frugal.tar.gz

fetch:
	@cd config/root && wget --mirror --convert-links --adjust-extension --page-requisites --no-parent --quiet -i ../websites.txt
