# satisfactory-operator

A Kubernetes operator that runs [Satisfactory](https://www.satisfactorygame.com/) Dedicated Servers. You declare a server as a custom resource; the operator deploys it, claims it, watches its health through the game's HTTPS API, and takes scheduled backups.

> **Status:** early development (`v1alpha1`). The API may change without notice and there are no published images or releases yet.

## Features

- **SatisfactoryServer** — deploys the game server as a single-replica StatefulSet with a persistent `/config` volume, a headless Service and an external Service (`LoadBalancer`, `NodePort` or `ClusterIP`).
- **Automatic claiming** — with an admin password Secret, the operator logs in, claims an unclaimed server, sets its name and caches the API token.
- **Health and status** — probes the game API and reports `Ready`, `Progressing`, `Degraded`, `ServerReachable`, `ServerClaimed`, `WorldLoaded` and `Authenticated` conditions, plus phase, server version and connected players.
- **SaveGame** — one-shot or cron-scheduled saves through the game API, with retention by count and age.
- **ClusterPolicy** — cluster-wide defaults (resources, storage class, server settings) and checks (allowed images, max servers per namespace). Violations are reported as Warning Events and on the policy's `Enforced` condition; they do not block servers.
- **PVC retention** — save data is kept when a server is deleted unless `retainPVCOnDelete: false`.

The game server image defaults to the community image [`ghcr.io/robtme/satisfactory-server`](https://github.com/robtme/satisfactory-server) (amd64 only). Game updates on pod restart are off by default (`skipUpdate: true`) to avoid unexpected 8 GB+ downloads.

## Install

Install the CRDs and the operator (multi-arch image, `linux/amd64` and `linux/arm64`) from the latest release:

```sh
# x-release-please-start-version
kubectl apply -f https://github.com/oliverziegert/satisfactory-operator/releases/download/v0.1.1/install.yaml
# x-release-please-end
```

Then create a server as in the last step of the quick start below.

## Quick start (from source)

Prerequisites: Go 1.25+, Docker, kubectl, and an **amd64** Kubernetes cluster with a default StorageClass. Plan for ~4–8 GiB memory and ~15 GiB storage per server.

```sh
# Build and push the operator image
make docker-build docker-push IMG=<registry>/satisfactory-operator:dev

# Install the CRDs and deploy the operator
make deploy IMG=<registry>/satisfactory-operator:dev

# Create the admin password Secret and a server
kubectl create secret generic my-satisfactory-server-admin --from-literal=password='<admin-password>'
kubectl apply -f config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml

kubectl get satisfactoryservers -w
```

Then enable `adminPasswordSecretRef` in the sample if you want the operator to claim the server for you. The first start downloads the game (several GB) and can take a while before the server becomes `Ready`.

## Example

```yaml
apiVersion: satisfactory.oliverziegert.github.com/v1alpha1
kind: SatisfactoryServer
metadata:
  name: my-satisfactory-server
spec:
  image: ghcr.io/robtme/satisfactory-server:v1.9.10
  maxPlayers: 4
  serviceType: LoadBalancer
  adminPasswordSecretRef:
    name: my-satisfactory-server-admin
  storage:
    size: 15Gi
---
apiVersion: satisfactory.oliverziegert.github.com/v1alpha1
kind: SaveGame
metadata:
  name: hourly-backup
spec:
  serverRef: my-satisfactory-server
  saveName: hourly
  schedule: "0 * * * *"
  retentionPolicy:
    maxCount: 24
    maxAge: 72h
```

Every field is documented in the samples under [`config/samples/`](config/samples/), or run `kubectl explain satisfactoryserver.spec`.

## Uninstall

```sh
kubectl delete satisfactoryservers,savegames --all
make undeploy
```

PVCs with save data stay behind unless the servers set `retainPVCOnDelete: false`.

## Development

```sh
make run        # run the controller locally against your current kubeconfig
make test       # unit and envtest tests
make lint       # golangci-lint
make test-e2e   # end-to-end tests in a temporary Kind cluster
make help       # all targets
```

After changing `api/v1alpha1/*_types.go`, run `make manifests generate` and commit the result; CI fails on stale generated files. Dependencies are kept current by Renovate (`.github/renovate.json5`).

Built with [Operator SDK](https://sdk.operatorframework.io/) / [Kubebuilder](https://book.kubebuilder.io/).

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
