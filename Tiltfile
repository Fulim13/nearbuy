load('ext://restart_process', 'docker_build_with_restart')

# ---------- Config ----------
k8s_yaml('./deploy/dev/k8s/app-config.yaml')                # Apply the app's ConfigMap + Secret
k8s_resource(
    new_name='configs',                                      # Name shown in the Tilt UI
    objects=['app-config:configmap', 'app-secrets:secret'],  # Bundle these k8s objects into it
    labels=['infra'],                                        # Group under "infra" in the UI
)

# ---------- Postgres ----------
k8s_yaml('./deploy/dev/k8s/postgres.yaml')                   # Apply the Postgres yaml
k8s_resource(
    'postgres',                                              # Tilt resource for the postgres workload
    port_forwards='5432:5432',                               # localhost:5432 -> pod, so you can run migrations from your laptop
    objects=['postgres-initdb:configmap'],                   # Attach the DB init-script ConfigMap to it
    labels=['infra'],                                        # Group under "infra"
)

# ---------- Redis ----------
k8s_yaml('./deploy/dev/k8s/redis.yaml')                      # Apply the Redis manifests
k8s_resource('redis', port_forwards='6379:6379', labels=['infra'])  # Expose Redis on localhost:6379, group under "infra"

# ---------- API Gateway image ----------
docker_build_with_restart(
    'nearbuy/api-gateway',                                  # Image name: must match the Deployment's image:
    '.',                                                     # Build context = repo root
    dockerfile='./services/api-gateway/Dockerfile',          # Which Dockerfile to use
    entrypoint=['/app/build/server'],                        # Command re-run after each live update
    only=[                                                   # Only these paths trigger rebuilds (others are ignored)
        './go.mod',                                          # Go module deps
        './go.sum',                                          # Go dep checksums
        './gen',                                             # Generated code
        './pkg',                                             # Shared packages
        './services/api-gateway',                            # This service's own code
    ],
    live_update=[                                            # Fast in-place updates instead of full rebuilds:
        fall_back_on(['./go.mod', './go.sum']),              # Deps changed? Do a full image rebuild instead
        sync('./services/api-gateway', '/app/services/api-gateway'),  # Copy service code into the running container
        sync('./pkg', '/app/pkg'),                           # Copy shared pkg code into the container
        run('go build -o /app/build/server ./services/api-gateway/cmd/server',  # Recompile the binary inside the container
            trigger=['./services/api-gateway', './pkg']),    # ...only when these folders change
    ],
)

# ---------- API Gateway k8s objects ----------
k8s_yaml([                                                   # Apply the gateway's manifests:
    './deploy/dev/k8s/api-gateway-deployment.yaml',          # Deployment (runs the pods)
    './deploy/dev/k8s/api-gateway-svc.yaml',                 # Service (stable in-cluster address)
    './deploy/dev/k8s/ingress.yaml',                         # Ingress (external HTTP routing)
])

k8s_resource(
    'api-gateway',                                           # Tilt resource for the gateway Deployment
    objects=['nearbuy-ingress:ingress'],                     # Attach the Ingress to it
    resource_deps=['postgres', 'redis'],                     # Wait for Postgres + Redis before starting
    labels=['services'],                                     # Group under "services"
)

# ---------- Migrations ----------
SERVICES_WITH_DB = ['user', 'shop', 'voucher', 'order', 'blog']

for svc in SERVICES_WITH_DB:
    local_resource(
        'migrate-' + svc,
        cmd='./scripts/migrate.sh %s up' % svc,
        deps=['./services/%s-service/database/migrations' % svc],
        resource_deps=['postgres'],
        labels=['init'],
    )

# sqlc regenerates when a query or a schema file changes. Keeping it in Tilt means
# you never debug a stale internal/repository.
local_resource(
    'sqlc',
    cmd='make sqlc',
    deps=['./services/%s-service/database' % s for s in SERVICES_WITH_DB],
    labels=['init'],
)
