CLUSTER       ?= nearbuy
INGRESS_NGINX ?= https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.11.2/deploy/static/provider/kind/deploy.yaml
SERVICES_WITH_DB = user shop voucher order blog

.PHONY: bootstrap cluster ingress up down nuke psql redis migrate-up migrate-down migrate-reset sqlc

bootstrap: cluster ingress

cluster:
	kind create cluster --config ./deploy/kind/cluster-nearbuy.yaml

ingress:
	kubectl apply -f $(INGRESS_NGINX)
	kubectl -n ingress-nginx wait --for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller --timeout=180s

up:
	tilt up

down:
	tilt down

nuke:
	kind delete cluster --name $(CLUSTER)

psql:
	psql "postgres://postgres:postgres@localhost:5432/postgres"

redis:
	redis-cli -h localhost -p 6379

migrate-up:
	@for s in $(SERVICES_WITH_DB); do CLUSTER=$(CLUSTER) ./scripts/migrate.sh $$s up; done

migrate-down:
	@for s in $(SERVICES_WITH_DB); do CLUSTER=$(CLUSTER) ./scripts/migrate.sh $$s down -all; done

migrate-reset: migrate-down migrate-up

sqlc:
      @for s in $(SERVICES_WITH_DB); do \
              if ! ls services/$$s-service/database/queries/*.sql >/dev/null 2>&1; then \
                      echo "==> sqlc $$s (no queries yet, skipping)"; continue; \
              fi; \
              echo "==> sqlc $$s"; (cd services/$$s-service && sqlc generate); \
      done
