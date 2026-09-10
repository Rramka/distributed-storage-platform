COMPOSE := docker compose -f deploy/compose/docker-compose.yml

.PHONY: up down ps logs test test-short vet fmt migrate build fleet-up fleet-down fleet-seed fleet-kill6

up:
	$(COMPOSE) up -d --wait postgres
	@$(MAKE) migrate
	$(COMPOSE) up -d --build
	@$(MAKE) export-ca

down:
	$(COMPOSE) down

fleet-up: up

fleet-down:
	$(COMPOSE) down -v

fleet-seed:
	$(COMPOSE) up fleet-seed --build

# Stop 6 of 16 agents for the M3 durability check (docs/10-mvp-roadmap.md § M3).
fleet-kill6:
	$(COMPOSE) kill agent11 agent12 agent13 agent14 agent15 agent16

export-ca:
	@mkdir -p .local
	@$(COMPOSE) cp metadata:/ca/ca.crt .local/ca.crt
	@echo "wrote .local/ca.crt (DSP_CA_FILE)"

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
	@for f in deploy/migrations/*.sql; do \
		echo "applying $$f"; \
		$(COMPOSE) exec -T postgres psql -U dsp -d dsp -f /docker-entrypoint-initdb.d/$$(basename $$f); \
	done
