run-load-test:
	@curl -X POST  127.0.0.1:8001/admin/purge-payments
	@curl -X POST  127.0.0.1:8002/admin/purge-payments
	K6_WEB_DASHBOARD=true \
	K6_WEB_DASHBOARD_PORT=5665 \
	K6_WEB_DASHBOARD_PERIOD=2s \
	k6 run ../__PESSOAL__/rinha-de-backend-2025/rinha-test/rinha.js
# 	 -e MAX_REQUESTS=3000 
# 	K6_MAX_REQUESTS=700 \
# 	K6_WEB_DASHBOARD_OPEN=true \

docker-compose-build-run-test:
	@docker compose -f ../__PESSOAL__/rinha-de-backend-2025/payment-processor/docker-compose.yml down
	@docker compose -f ../__PESSOAL__/rinha-de-backend-2025/payment-processor/docker-compose.yml up -d --build --remove-orphans --force-recreate
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


