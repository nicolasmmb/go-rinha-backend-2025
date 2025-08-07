run-load-test:
	K6_WEB_DASHBOARD=true \
	K6_WEB_DASHBOARD_PORT=5665 \
	K6_WEB_DASHBOARD_PERIOD=2s \
	K6_WEB_DASHBOARD_OPEN=true \
	k6 run -e MAX_REQUESTS=700 ../rinha-test/rinha.js

docker-build-compose:
	@docker compose -f .infra/docker-compose.yaml up --build -d --remove-orphans

# docker-build-image:
# 	@docker build -t nicolasmmb:latest -f ./.infra/Dockerfile .

# docker-push-image:
# 	@docker push nicolasmmb/rinha-go-2025:latest


