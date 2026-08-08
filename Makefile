SHELL := /bin/sh

ENV ?= local
COMPOSE_ENV_FILE := deploy/env/$(ENV).env
COMPOSE_FILES := -f docker-compose.yml -f docker-compose.$(ENV).yml
COMPOSE := docker compose --env-file $(COMPOSE_ENV_FILE) $(COMPOSE_FILES)

.PHONY: deploy down dev logs config

validate-env:
	@test "$(ENV)" = "local" || test "$(ENV)" = "dev" || test "$(ENV)" = "prod" || (echo "ENV must be local, dev, or prod" && exit 1)
	@test -f "$(COMPOSE_ENV_FILE)" || (echo "Missing $(COMPOSE_ENV_FILE)" && exit 1)

deploy: validate-env
	$(COMPOSE) up -d --build
	@echo "Fantasy Helper $(ENV) is running. Use 'make logs ENV=$(ENV)' to follow output."

down: validate-env
	$(COMPOSE) down --remove-orphans

dev:
	$(MAKE) deploy ENV=local

logs: validate-env
	$(COMPOSE) logs -f

config: validate-env
	$(COMPOSE) config

