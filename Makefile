PROJECT_NAME:=tcp_proxy
GOLANGCI_LINT := $(shell command -v golangci-lint 2> /dev/null)
CURRENT_USER := $(shell whoami)
CURDIR := $(shell pwd)

# test coverage threshold
COVERAGE_THRESHOLD:=30
COVERAGE_TOTAL := $(shell go tool cover -func=cover.out | grep total | grep -Eo '[0-9]+\.[0-9]+')
COVERAGE_PASS_THRESHOLD := $(shell echo "$(COVERAGE_TOTAL) $(COVERAGE_THRESHOLD)" | awk '{print ($$1 >= $$2)}')

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

install-lint: ## Installs golangci-lint tool which a go linter
ifndef GOLANGCI_LINT
	${info golangci-lint not found, installing golangci-lint@latest}
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
endif

test: ## Runs tests
	${info Running tests...}
	go test -v -race ./... -cover -coverprofile cover.out
	go tool cover -func cover.out | grep total

vulcheck: ## Runs vulnerability check
	${info Running vulnerability check...}
	govulncheck ./...

lint: install-lint ## Runs linters
	@echo "-- linter running"
	golangci-lint run -c .golangci.yaml ./internal...
	golangci-lint run -c .golangci.yaml ./cmd...

build: ## Builds binary
	@echo "-- building binary"
	go build -ldflags="-X 'main.confFile=$(CURDIR)/configs/app_conf.yml'" -o ./bin/binary ./cmd

patch_sudoers: ## Patch sudoers file
	@echo "-- patching sudoers"
	sudo echo "$(CURRENT_USER) ALL=(ALL) NOPASSWD: /bin/systemctl start proxier.service" >> /etc/sudoers
	sudo echo "$(CURRENT_USER) ALL=(ALL) NOPASSWD: /bin/systemctl stop proxier.service" >> /etc/sudoers
	sudo echo "$(CURRENT_USER) ALL=(ALL) NOPASSWD: /bin/systemctl restart proxier.service" >> /etc/sudoers
	sudo echo "$(CURRENT_USER) ALL=(ALL) NOPASSWD: /bin/journalctl -u proxier.service -f" >> /etc/sudoers

coverage: ## Check test coverage is enough
	@echo "Threshold:                ${COVERAGE_THRESHOLD}%"
	@echo "Current test coverage is: ${COVERAGE_TOTAL}%"
	@if [ "${COVERAGE_PASS_THRESHOLD}" -eq "0" ] ; then \
		echo "Test coverage is lower than threshold"; \
		exit 1; \
	fi

deploy: ## Deploy systemd service
	git pull origin master
	go mod download
	make build
	sudo systemctl stop proxier.service
	sudo systemctl start proxier.service

self_deploy: ## Deploy self from application
	@echo "-- deploying self"
	git pull origin master
	go mod download
	make build

logs: ## Show logs of service
	sudo journalctl -u proxier.service -f -o cat

install_service: patch_sudoers ## Install service
	git config --global --add safe.directory $(CURDIR)
	@echo "-- creating service"
	sudo mkdir -p /etc/systemd/system
	cp proxier.service proxier.service.local
	@sed -i 's|ExecStart=/path_to_binary|ExecStart=$(shell pwd)/bin/binary|' proxier.service.local
	@sed -i 's|^User=.*|User=$(shell whoami)|' proxier.service.local
	sudo cp proxier.service.local /etc/systemd/system/proxier.service
	@echo "-- enable service"
	sudo systemctl start proxier && sudo systemctl enable proxier

.PHONY: help install-lint test gogen lint build run vulcheck coverage patch_sudoers deploy logs install_service self_deploy
.DEFAULT_GOAL := help