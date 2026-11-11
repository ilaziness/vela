# Makefile for Vela (go-gui desktop)
VERSION ?= v1.0.0
MODE ?= release
APP_NAME ?= Vela
# -s -w: strip symbol table + DWARF (smaller release binary)
LDFLAGS := -s -w -X github.com/ilaziness/vela/internal/buildinfo.Version=$(VERSION) -X github.com/ilaziness/vela/internal/buildinfo.Mode=$(MODE)

.PHONY: help build run test lint clean

.DEFAULT_GOAL := help

help:
	@echo "Targets: build | run | test | lint | clean"

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) .

run: build
	./bin/$(APP_NAME)

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin
