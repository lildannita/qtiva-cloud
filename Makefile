SHELL := /usr/bin/env bash
.DEFAULT_GOAL 	:= help

COMPOSE_FILE 	:= docker-compose.yml
PG_CONTAINER 	:= qtiva-postgres

BIN_DIR 		:= bin
MANAGER_BIN 	:= $(BIN_DIR)/qtiva-manager
AGENT_BIN   	:= $(BIN_DIR)/qtiva-agent

.PHONY: help create-env up down restart ps logs psql migrate tidy build clean \
        run-manager run-agent update-admin health-manager health-agent

help: ## Показать список команд
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

create-env: ## Создать .env из .env.example (если .env отсутствует)
	@if [ -f .env ]; then \
		echo ".env уже существует"; \
	else \
		cp .env.example .env && echo "Создан .env (скопирован из .env.example)"; \
	fi

up: ## Поднять контейнеры
	@set -a && source .env && set +a && \
	docker compose -f $(COMPOSE_FILE) up -d

down: ## Остановить контейнеры
	docker compose -f $(COMPOSE_FILE) down

restart: down up ## Перезапустить контейнеры

ps: ## Показать статус контейнеров compose
	docker compose -f $(COMPOSE_FILE) ps

logs: ## Логи Postgres
	docker logs -f $(PG_CONTAINER)

psql: ## Зайти в psql внутри контейнера
	@set -a && source .env && set +a && \
	docker exec -it $(PG_CONTAINER) psql -U $$POSTGRES_USER -d $$POSTGRES_DB

migrate: ## Применить миграции
	@chmod +x ./scripts/migrate.sh
	@set -a && source .env && set +a && \
	./scripts/migrate.sh

tidy:
	go mod tidy

build: ## Собрать бинарники
	@chmod +x ./scripts/build.sh
	@set -a && source .env && set +a && \
	./scripts/build.sh

clean: ## Удалить ./bin
	rm -rf $(BIN_DIR)

run-manager: ## Запустить manager
	@if [ ! -f "$(MANAGER_BIN)" ]; then \
		echo "Исполняемый файл $(MANAGER_BIN) не найден"; \
		exit 1; \
	fi
	./$(MANAGER_BIN) serve

run-agent: ## Запустить agent
	@if [ ! -f "$(AGENT_BIN)" ]; then \
		echo "Исполняемый файл $(AGENT_BIN) не найден"; \
		exit 1; \
	fi
	./$(AGENT_BIN) serve

health-manager: ## Проверить /health у manager
	curl -sS http://127.0.0.1:8080/health

health-agent: ## Проверить /health у agent
	curl -sS http://127.0.0.1:8090/health

update-admin: ## Создать или обновить администратора из переменных окружения
	@set -a && source .env && set +a && \
	./$(MANAGER_BIN) \
		seed-admin --replace \
		--email "$$QTIVA_ADMIN_EMAIL" \
		--password "$$QTIVA_ADMIN_PASSWORD"
