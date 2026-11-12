# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent

BIN     := ds-agent
PKG     := ./cmd/ds-agent
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help
.PHONY: help build test lint secrets banc e2e check run clean fmt

help: ## affiche cette aide
	@grep -E '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*## "}; {printf "  make %-8s %s\n", $$1, $$2}'

build: ## compile le binaire (version = git describe)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

run: build ## lance l'agent
	./$(BIN)

fmt: ## formate le code
	gofmt -l -w .

lint: ## gofmt + go vet
	@test -z "$$(gofmt -l .)" || { echo "fichiers à formater :"; gofmt -l .; exit 1; }
	go vet ./...

test: ## tests unitaires
	go test -count=1 ./...

secrets: ## aucun jeton ni fichier interdit versionné
	./scripts/check-secrets.sh

banc: ## banc hostile (16 scénarios, hors ligne)
	bash tests/paranoid.sh

e2e: ## bout-en-bout (binaire réel + mock)
	bash tests/e2e.sh

check: lint test secrets banc e2e ## tout ce qui doit être vert avant une PR

clean: ## binaire, sessions, journal d'audit
	./clean.sh
