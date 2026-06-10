.PHONY: dev dev-file dev-sqlite build migrate seed infra-up infra-down ollama-pull ollama-pull-local docker-build

dev:
	go run ./cmd/mcphub

dev-file:
	LLM_STORE_DSN=file://./data/store.yml go run ./cmd/mcphub

dev-sqlite:
	LLM_STORE_DSN=sqlite://./data/llm.db go run ./cmd/mcphub

build:
	go build -o bin/mcphub ./cmd/mcphub
	go build -o bin/migrate ./cmd/migrate

migrate:
	go run ./cmd/migrate

seed:
	sh deploy/seed-demo.sh

infra-up:
	docker compose -f deploy/docker-compose.yml up -d postgres ollama whisper

infra-down:
	docker compose -f deploy/docker-compose.yml down

# Pull local dev models: text (qwen q4) + vision (llama3.2-vision)
ollama-pull-local:
	docker compose -f deploy/docker-compose.yml exec ollama ollama pull $(or $(TEXT_MODEL),qwen2.5:7b-instruct-q4_K_M)
	docker compose -f deploy/docker-compose.yml exec ollama ollama pull $(or $(VISION_MODEL),llama3.2-vision)

ollama-pull: ollama-pull-local

docker-build:
	docker compose -f deploy/docker-compose.yml build

up:
	docker compose -f deploy/docker-compose.yml up -d --build
