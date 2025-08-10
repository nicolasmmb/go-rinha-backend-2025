run-load-test:
	K6_WEB_DASHBOARD=true \
	K6_WEB_DASHBOARD_PORT=5665 \
	K6_WEB_DASHBOARD_PERIOD=2s \
	K6_WEB_DASHBOARD_OPEN=true \
	k6 run -e MAX_REQUESTS=700 ../__PESSOAL__/rinha-de-backend-2025/rinha-test/rinha.js

docker-compose-build-run-test:
	@docker compose -f .infra/docker-compose.yaml down
	@docker compose -f .infra/docker-compose.yaml up --build -d --remove-orphans --force-recreate
	@for i in {1..10}; do \
		echo "Waiting for the server to start..."; \
		status=$$(curl -s -o /dev/null -w "%{http_code}" -X POST 127.0.0.1:9999/reset || true); \
		if [ "$$status" = "200" ]; then \
			echo "Server is ready!"; \
			break; \
		fi; \
		sleep 1; \
	done
	make run-load-test


