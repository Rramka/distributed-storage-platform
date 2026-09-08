COMPOSE := docker compose -f deploy/compose/docker-compose.yml

.PHONY: up down ps logs test test-short vet fmt migrate build

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f --tail=100

build:
	go build ./...

test:
	go test ./...

test-short:
	go test ./... -short

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')
	@if command -v goimports >/dev/null 2>&1; then goimports -w $$(find . -name '*.go' -not -path './.git/*'); fi

vet:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "$$unformatted"; exit 1; fi
	go vet ./...

migrate:
	$(COMPOSE) exec -T postgres psql -U dsp -d dsp -f /docker-entrypoint-initdb.d/000001_init.sql
