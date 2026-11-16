# Makefile for Vela (go-gui desktop)
VERSION ?= v1.0.0
MODE ?= release
APP_NAME ?= Vela
GIT_HASH ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
# -s -w: strip symbol table + DWARF (smaller release binary)
LDFLAGS := -s -w -X github.com/ilaziness/vela/internal/buildinfo.Version=$(VERSION) -X github.com/ilaziness/vela/internal/buildinfo.Mode=$(MODE) -X github.com/ilaziness/vela/internal/buildinfo.GitInfo=$(GIT_HASH) -X github.com/ilaziness/vela/internal/buildinfo.BuildTime=$(BUILD_TIME)

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
