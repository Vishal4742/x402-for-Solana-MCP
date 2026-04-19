SHELL := /bin/bash

.PHONY: up down gateway verifier sdk-build

up:
	docker compose up -d

down:
	docker compose down

gateway:
	cd services/gateway && go run ./cmd/gateway

verifier:
	cd services/verifier && go run ./cmd/verifier

sdk-build:
	pnpm --filter @x402/sdk build

