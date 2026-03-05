ENV     ?= local
APP     := skyrouter
BIN_DIR := bin

# prod uses docker build directly — no compose file needed
ifneq ($(ENV),prod)
COMPOSE := docker compose \
	--project-directory . \
	--project-name $(APP)-$(ENV) \
	-f deploy/docker-compose.$(ENV).yml
endif

.PHONY: run down build test migrate teardown generate tidy logs clean job help

run: ## Start services (postgres + app with hot reload)
	$(COMPOSE) up -d --build

down: ## Stop services and remove volumes
	$(COMPOSE) down -v

build: ## Build binary → ./bin/server (ENV=local|ci) or Docker image (ENV=prod)
ifeq ($(ENV),prod)
	docker build --target final -t $(APP):latest .
else
	mkdir -p $(BIN_DIR)
	$(COMPOSE) run --rm build
endif

test: ## Run tests inside Docker with live postgres
	$(COMPOSE) run --rm test

migrate: ## Apply all pending migrations
	$(COMPOSE) run --rm migrate up

teardown: ## Roll back all migrations
	$(COMPOSE) run --rm migrate down -all

generate: ## Regenerate Bob models and Mockery mocks (requires ENV=local with postgres running)
	find . \( -name "*.bob.go" -o -name "*.bob_test.go" \) -delete
	$(COMPOSE) run --rm bobgen
	go tool mockery

job: ## Run a job locally against postgres (JOB=fetch-waypoints)
	$(COMPOSE) run --rm job $(JOB)

tidy: ## Download and tidy Go modules (run once after cloning or adding deps)
	docker run --rm \
		-v $(CURDIR):/app \
		-w /app \
		golang:1.25-alpine \
		go mod tidy

logs: ## Tail logs for all services
	$(COMPOSE) logs -f

clean: ## Remove all containers, volumes, images, and local binaries
	$(COMPOSE) down -v --rmi local
	rm -rf $(BIN_DIR)

help: ## Show this help
	@grep -E '^[a-z]+:.*## ' Makefile | \
		awk -F':.*## ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
