DC=docker-compose -f docker-compose.yaml

.PHONY: help
.DEFAULT_GOAL := help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

stop: ## Stop all Docker Containers run in Compose
	$(DC) stop

clean: stop ## Clean all Docker Containers and Volumes
	$(DC) down --rmi local --remove-orphans -v
	$(DC) rm -f -v

build: clean ## Rebuild the Docker Image for use by Compose
	$(DC) build

run: stop ## Run the Application
	$(DC) up
