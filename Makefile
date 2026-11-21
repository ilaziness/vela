# Makefile for Vela (go-gui desktop)
VERSION ?= v1.0.0
MODE ?= release
APP_NAME ?= Vela
GIT_HASH ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
# -s -w: strip symbol table + DWARF (smaller release binary)
LDFLAGS := -s -w -X github.com/ilaziness/vela/internal/buildinfo.Version=$(VERSION) -X github.com/ilaziness/vela/internal/buildinfo.Mode=$(MODE) -X github.com/ilaziness/vela/internal/buildinfo.GitInfo=$(GIT_HASH) -X github.com/ilaziness/vela/internal/buildinfo.BuildTime=$(BUILD_TIME)
# Release builds drop go-gui's F12 inspector overlay (prod build tag);
# MODE=debug keeps it.
TAGS := $(if $(filter release,$(MODE)),-tags prod,)
# -H windowsgui links the GUI subsystem so the exe doesn't spawn an empty
# console window. Drop it when you need stdout/stderr/panics for debugging.
ifeq ($(OS),Windows_NT)
HOST_GUIFLAG := -H windowsgui
endif

.PHONY: help build run test lint clean

.DEFAULT_GOAL := help

help:
	@echo "Targets: build | run | test | lint | clean"

build:
	mkdir -p bin
	go build -trimpath $(TAGS) -ldflags "$(LDFLAGS) $(HOST_GUIFLAG)" -o bin/$(APP_NAME) .

run: build
	./bin/$(APP_NAME)

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin
