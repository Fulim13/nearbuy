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
SERVICES_WITH_DB = ['user', 'shop', 'voucher', 'order']      # Services that own a database (and need migrations)

for svc in SERVICES_WITH_DB:                                # One migrate resource per service
    local_resource(                                          # Runs a shell command on your laptop, not in k8s
        'migrate-' + svc,                                    # Name shown in the Tilt UI, e.g. migrate-user
        cmd='./scripts/migrate.sh %s up' % svc,              # Apply all pending migrations for this service
        deps=['./services/%s-service/database/migrations' % svc],  # Re-run when a migration file is added or changed
        resource_deps=['postgres'],                          # Wait for Postgres (and its port-forward) first
        labels=['init'],                                     # Group under "init"
    )

# sqlc regenerates when a query or a schema file changes. Keeping it in Tilt means
# you never debug a stale internal/repository.
local_resource(
    'sqlc',                                                  # Name shown in the Tilt UI
    cmd='make sqlc',                                         # Regenerate Go code from the SQL queries
    deps=['./services/%s-service/database' % s for s in SERVICES_WITH_DB],  # Watch every service's queries + migrations
    labels=['init'],                                         # Group under "init"
)


# ---------- User Service ----------
docker_build_with_restart(                                   # Build the image; restart the binary on live updates
    'nearbuy/user-service',                                  # Image name: must match the Deployment's image:
    '.',                                                     # Build context = repo root
    dockerfile='./services/user-service/Dockerfile',         # Which Dockerfile to use
    entrypoint=['/app/build/server'],                        # Command re-run after each live update
    only=[                                                   # Only these paths trigger rebuilds (others are ignored)
        './go.mod',                                          # Go module deps
        './go.sum',                                          # Go dep checksums
        './gen',                                             # Generated code
        './pkg',                                             # Shared packages
        './services/user-service',                           # This service's own code
    ],
    live_update=[                                            # Fast in-place updates instead of full rebuilds:
        fall_back_on(['./go.mod', './go.sum']),              # Deps changed? Do a full image rebuild instead
        sync('./services/user-service', '/app/services/user-service'),  # Copy service code into the running container
        sync('./gen', '/app/gen'),                           # Copy generated code into the container
        sync('./pkg', '/app/pkg'),                           # Copy shared pkg code into the container
        run('go build -o /app/build/server ./services/user-service/cmd/server',  # Recompile the binary inside the container
            trigger=['./services/user-service', './gen', './pkg']),  # ...only when these folders change
    ],
)

k8s_yaml([                                                   # Apply the user service's manifests:
    './deploy/dev/k8s/user-service-deployment.yaml',         # Deployment (runs the pods)
    './deploy/dev/k8s/user-service-svc.yaml',                # Service (stable in-cluster address)
])
k8s_resource(
    'user-service',                                          # Tilt resource for the user-service Deployment
    resource_deps=['postgres', 'migrate-user'],              # Wait for Postgres + its migrations before starting
    labels=['services'],                                     # Group under "services"
)

# ---------- Shop Service ----------
docker_build_with_restart(                                   # Build the image; restart the binary on live updates
    'nearbuy/shop-service',                                  # Image name: must match the Deployment's image:
    '.',                                                     # Build context = repo root
    dockerfile='./services/shop-service/Dockerfile',         # Which Dockerfile to use
    entrypoint=['/app/build/server'],                        # Command re-run after each live update
    only=[                                                   # Only these paths trigger rebuilds (others are ignored)
        './go.mod',                                          # Go module deps
        './go.sum',                                          # Go dep checksums
        './gen',                                             # Generated code
        './pkg',                                             # Shared packages
        './services/shop-service',                           # This service's own code
    ],
    live_update=[                                            # Fast in-place updates instead of full rebuilds:
        fall_back_on(['./go.mod', './go.sum']),              # Deps changed? Do a full image rebuild instead
        sync('./services/shop-service', '/app/services/shop-service'),  # Copy service code into the running container
        sync('./gen', '/app/gen'),                           # Copy generated code into the container
        sync('./pkg', '/app/pkg'),                           # Copy shared pkg code into the container
        run('go build -o /app/build/server ./services/shop-service/cmd/server',  # Recompile the binary inside the container
            trigger=['./services/shop-service', './gen', './pkg']),  # ...only when these folders change
    ],
)

k8s_yaml([                                                   # Apply the shop service's manifests:
    './deploy/dev/k8s/shop-service-deployment.yaml',         # Deployment (runs the pods)
    './deploy/dev/k8s/shop-service-svc.yaml',                # Service (stable in-cluster address)
])
k8s_resource(
'shop-service',                                          		     # Tilt resource for the shop-service Deployment
    resource_deps=['postgres', 'redis', 'kafka-topics', 'migrate-shop'],     # Wait for Postgres, Redis + its migrations before starting
    labels=['services'],                                     	 	     # Group under "services"
)

# ---------- Kafka ----------
k8s_yaml('./deploy/dev/k8s/kafka.yaml')                      # Apply the Kafka Service + StatefulSet
k8s_resource(
    'kafka',                                                 # Tilt resource for the kafka StatefulSet
    port_forwards='9092:9092',                               # localhost:9092 -> pod; laptop also needs "127.0.0.1 kafka" in /etc/hosts
    labels=['infra'],                                        # Group under "infra"
)

k8s_yaml('./deploy/dev/k8s/kafka-topics-job.yaml')           # Apply the Job that creates the topics
k8s_resource(
    'kafka-topics',                                          # Tilt resource for the topic-creating Job
    resource_deps=['kafka'],                                 # Wait for Kafka before creating topics
    labels=['init'],                                         # Group under "init"
)


# ---------- Voucher Service ----------
docker_build_with_restart(                                   # Build the image; restart the binary on live updates
    'nearbuy/voucher-service',                               # Image name: must match the Deployment's image:
    '.',                                                     # Build context = repo root
    dockerfile='./services/voucher-service/Dockerfile',      # Which Dockerfile to use
    entrypoint=['/app/build/server'],                        # Command re-run after each live update
    only=[                                                   # Only these paths trigger rebuilds (others are ignored)
        './go.mod',                                          # Go module deps
        './go.sum',                                          # Go dep checksums
        './gen',                                             # Generated code
        './pkg',                                             # Shared packages
        './services/voucher-service',                        # This service's own code
    ],
    live_update=[                                            # Fast in-place updates instead of full rebuilds:
        fall_back_on(['./go.mod', './go.sum']),              # Deps changed? Do a full image rebuild instead
        sync('./services/voucher-service', '/app/services/voucher-service'),  # Copy service code into the running container
        sync('./gen', '/app/gen'),                           # Copy generated code into the container
        sync('./pkg', '/app/pkg'),                           # Copy shared pkg code into the container
        run('go build -o /app/build/server ./services/voucher-service/cmd/server',  # Recompile the binary inside the container
            trigger=['./services/voucher-service', './gen', './pkg']),  # ...only when these folders change
    ],
)

k8s_yaml([                                                   # Apply the voucher service's manifests:
    './deploy/dev/k8s/voucher-service-deployment.yaml',      # Deployment (runs the pods)
    './deploy/dev/k8s/voucher-service-svc.yaml',             # Service (stable in-cluster address)
])
k8s_resource(
    'voucher-service',                                       # Tilt resource for the voucher-service Deployment
    resource_deps=['postgres', 'redis', 'kafka-topics', 'migrate-voucher'],  # Wait for Postgres, Redis + its migrations before starting
    labels=['services'],                                     # Group under "services"
)
