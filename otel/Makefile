DC=docker compose

.PHONY: help
.DEFAULT_GOAL := help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

stop: ## Stop the containers
	$(DC) stop

clean: ## Remove the containers, volumes and local images
	$(DC) down --rmi local --remove-orphans -v

build: ## Build the podinfo images
	$(DC) build

run: ## Build and start the stack, wait until it is ready
	$(DC) up -d --build --wait
