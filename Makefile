CLUSTER       ?= nearbuy
INGRESS_NGINX ?= https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.11.2/deploy/static/provider/kind/deploy.yaml


.PHONY: bootstrap cluster ingress up down nuke psql redis

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
