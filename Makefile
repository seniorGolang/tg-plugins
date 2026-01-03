.PHONY: build install clean all help check-deps

# Директория для установки плагинов (по умолчанию ~/.config/tg/plugins)
PLUGINS_DIR ?= $(shell echo $$HOME)/.config/tg/plugins

# Утилиты
JQ := jq
GO := go

# Проверка зависимостей
check-deps: ## Проверить наличие необходимых утилит
	@command -v $(JQ) >/dev/null 2>&1 || { echo "$(RED)ОШИБКА: jq не установлен. Установите: brew install jq$(NC)"; exit 1; }
	@command -v $(GO) >/dev/null 2>&1 || { echo "$(RED)ОШИБКА: go не установлен$(NC)"; exit 1; }
	@echo "$(GREEN)Все зависимости установлены$(NC)"

# Цвета для вывода
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'

all: check-deps build install ## Собрать и установить все плагины

build: check-deps ## Собрать все плагины
	@echo "$(YELLOW)Сборка плагинов...$(NC)"
	@for plugin_dir in plugins/*/; do \
		if [ -d "$$plugin_dir" ]; then \
			plugin_name=$$(basename "$$plugin_dir"); \
			echo "$(GREEN)Сборка плагина: $$plugin_name$(NC)"; \
			cd "$$plugin_dir" && $(GO) generate tgp.go && cd ../.. || exit 1; \
		fi \
	done
	@echo "$(GREEN)Все плагины собраны$(NC)"

install: check-deps ## Установить все плагины в $(PLUGINS_DIR)
	@echo "$(YELLOW)Установка плагинов в $(PLUGINS_DIR)...$(NC)"
	@mkdir -p "$(PLUGINS_DIR)"
	@for plugin_dir in plugins/*/; do \
		if [ -d "$$plugin_dir" ]; then \
			plugin_dir_name=$$(basename "$$plugin_dir"); \
			plugin_json="$$plugin_dir/plugin.json"; \
			if [ ! -f "$$plugin_json" ]; then \
				echo "$(RED)ОШИБКА: plugin.json не найден в $$plugin_dir$(NC)"; \
				continue; \
			fi; \
			plugin_name=$$($(JQ) -r '.name' "$$plugin_json"); \
			plugin_version=$$($(JQ) -r '.version' "$$plugin_json"); \
			if [ -z "$$plugin_name" ] || [ "$$plugin_name" = "null" ]; then \
				echo "$(RED)ОШИБКА: Имя плагина не найдено в $$plugin_json$(NC)"; \
				continue; \
			fi; \
			if [ -z "$$plugin_version" ] || [ "$$plugin_version" = "null" ]; then \
				echo "$(RED)ОШИБКА: Версия плагина не найдена в $$plugin_json$(NC)"; \
				continue; \
			fi; \
			echo "$(GREEN)Установка плагина: $$plugin_name версии $$plugin_version$(NC)"; \
			install_dir="$(PLUGINS_DIR)/$$plugin_name/$$plugin_version"; \
			mkdir -p "$$install_dir"; \
			tgp_file=""; \
			sha256_file=""; \
			json_file=""; \
			if [ -f "dist/$$plugin_name.tgp" ]; then \
				tgp_file="dist/$$plugin_name.tgp"; \
				sha256_file="dist/$$plugin_name.sha256"; \
				json_file="dist/$$plugin_name.json"; \
			elif [ -f "$$plugin_dir/dist/$$plugin_name.tgp" ]; then \
				tgp_file="$$plugin_dir/dist/$$plugin_name.tgp"; \
				sha256_file="$$plugin_dir/dist/$$plugin_name.sha256"; \
				json_file="$$plugin_dir/dist/$$plugin_name.json"; \
			elif [ -f "$$plugin_dir/$$plugin_name.tgp" ]; then \
				tgp_file="$$plugin_dir/$$plugin_name.tgp"; \
				sha256_file="$$plugin_dir/$$plugin_name.sha256"; \
				json_file="$$plugin_dir/$$plugin_name.json"; \
			fi; \
			if [ ! -f "$$tgp_file" ]; then \
				echo "$(RED)ОШИБКА: $$tgp_file не найден$(NC)"; \
				continue; \
			fi; \
			if [ ! -f "$$sha256_file" ]; then \
				echo "$(RED)ОШИБКА: $$sha256_file не найден$(NC)"; \
				continue; \
			fi; \
			if [ ! -f "$$json_file" ]; then \
				echo "$(YELLOW)Предупреждение: $$json_file не найден, используем plugin.json$(NC)"; \
				json_file="$$plugin_json"; \
			fi; \
			cp "$$tgp_file" "$$install_dir/$$plugin_name.tgp"; \
			cp "$$sha256_file" "$$install_dir/$$plugin_name.sha256"; \
			cp "$$json_file" "$$install_dir/$$plugin_name.json"; \
			cp "$$plugin_json" "$$install_dir/plugin.json"; \
			metadata_json="$$install_dir/metadata.json"; \
			repo_url="file://$$(pwd)"; \
			installed_at=$$(date +%s); \
			$(JQ) -n --arg repo "$$repo_url" --arg type "local" --argjson time "$$installed_at" --slurpfile info "$$plugin_json" '{repositoryURL: $$repo, installedAt: ($$time | tonumber), sourceType: $$type, pluginInfo: $$info[0]}' > "$$metadata_json"; \
			echo "$(GREEN)  ✓ Установлен в $$install_dir$(NC)"; \
		fi \
	done
	@echo "$(GREEN)Все плагины установлены$(NC)"

clean: ## Очистить собранные файлы
	@echo "$(YELLOW)Очистка...$(NC)"
	@for plugin_dir in plugins/*/; do \
		if [ -d "$$plugin_dir" ]; then \
			plugin_name=$$(basename "$$plugin_dir"); \
			echo "$(YELLOW)Очистка $$plugin_name...$(NC)"; \
			rm -rf "$$plugin_dir/dist"; \
		fi \
	done
	@rm -rf dist
	@echo "$(GREEN)Очистка завершена$(NC)"

install-server: ## Установить только плагин server
	@echo "$(YELLOW)Установка плагина server...$(NC)"
	@$(MAKE) install PLUGIN_FILTER=server

.PHONY: list
list: ## Показать список плагинов
	@echo "$(GREEN)Доступные плагины:$(NC)"
	@for plugin_dir in plugins/*/; do \
		if [ -d "$$plugin_dir" ]; then \
			plugin_dir_name=$$(basename "$$plugin_dir"); \
			plugin_json="$$plugin_dir/plugin.json"; \
			if [ -f "$$plugin_json" ]; then \
				plugin_name=$$($(JQ) -r '.name' "$$plugin_json"); \
				plugin_version=$$($(JQ) -r '.version' "$$plugin_json"); \
				echo "  $(GREEN)$$plugin_name$(NC) v$$plugin_version (директория: $$plugin_dir_name)"; \
			else \
				echo "  $(YELLOW)$$plugin_dir_name$(NC) (plugin.json не найден)"; \
			fi; \
		fi \
	done

