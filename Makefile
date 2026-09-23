COMPOSE := docker compose -f deploy/compose/docker-compose.yml

.PHONY: up down ps logs test test-short vet fmt migrate migrate-url build fleet-up fleet-down fleet-seed fleet-kill6 chaos-m4 chaos-partition demo invariants soak

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

# Stop 6 actual holders of a committed file (docs/10-mvp-roadmap.md § M4).
fleet-kill6:
	go run ./cmd/harness kill -n 6

chaos-m4:
	go run ./cmd/harness m4

chaos-partition:
	go run ./cmd/harness partition --region $${REGION:-eu-west} --for $${DURATION:-8m} --heal

demo:
	go run ./cmd/harness demo

invariants:
	go run ./cmd/harness invariants

soak:
	go run ./cmd/harness soak --duration $${DURATION:-2h}

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
		$(COMPOSE) exec -T postgres psql -U dsp -d dsp -v ON_ERROR_STOP=1 -f /docker-entrypoint-initdb.d/$$(basename $$f); \
	done

migrate-url:
	@test -n "$(POSTGRES_URL)" || (echo "POSTGRES_URL required" && exit 1)
	./scripts/apply-migrations.sh "$(POSTGRES_URL)"
