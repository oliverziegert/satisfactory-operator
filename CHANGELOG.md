# Changelog

## [0.2.1](https://github.com/oliverziegert/satisfactory-operator/compare/v0.2.0...v0.2.1) (2026-10-25)


### Bug Fixes

* **chart:** set appVersion on release ([#54](https://github.com/oliverziegert/satisfactory-operator/issues/54)) ([afa2e88](https://github.com/oliverziegert/satisfactory-operator/commit/afa2e886e958ac0ee265bcaf835377e65a5b2ed8))

## [0.2.0](https://github.com/oliverziegert/satisfactory-operator/compare/v0.1.4...v0.2.0) (2026-10-25)


### Features

* add Helm chart ([#52](https://github.com/oliverziegert/satisfactory-operator/issues/52)) ([a9e5d07](https://github.com/oliverziegert/satisfactory-operator/commit/a9e5d07cdad39d20b6c311ccfde04824db1b9302))
* add namespace-scoped watch mode ([#50](https://github.com/oliverziegert/satisfactory-operator/issues/50)) ([4b322a1](https://github.com/oliverziegert/satisfactory-operator/commit/4b322a1230a6a8109bf62beeb974bec3f4304070))


### Bug Fixes

* **deps:** update gcr.io/distroless/static:nonroot docker digest to 18f4cc2 ([#49](https://github.com/oliverziegert/satisfactory-operator/issues/49)) ([e1a1556](https://github.com/oliverziegert/satisfactory-operator/commit/e1a1556fc788450bc320ad96fc27077c45e55b9f))
* **deps:** update k8s.io/utils digest to ca04ba5 ([#46](https://github.com/oliverziegert/satisfactory-operator/issues/46)) ([54b105f](https://github.com/oliverziegert/satisfactory-operator/commit/54b105fded5f8f65727dff78e057eafd1cee555a))
* let `ClusterPolicy` server defaults apply ([#51](https://github.com/oliverziegert/satisfactory-operator/issues/51)) ([2750f48](https://github.com/oliverziegert/satisfactory-operator/commit/2750f482ab172d37ddb0c51b034830d164eb99a0))


### Miscellaneous Chores

* **deps:** update docker/login-action digest to 81c6f93 ([#48](https://github.com/oliverziegert/satisfactory-operator/issues/48)) ([c4e8d0b](https://github.com/oliverziegert/satisfactory-operator/commit/c4e8d0b98df1e76ce38bd910ebc751901907551d))
* **openspec:** archive add-helm-chart and add-watch-namespaces changes ([#53](https://github.com/oliverziegert/satisfactory-operator/issues/53)) ([1f37056](https://github.com/oliverziegert/satisfactory-operator/commit/1f3705674932ff90f961510deb36654c7512caaa))

## [0.1.4](https://github.com/oliverziegert/satisfactory-operator/compare/v0.1.3...v0.1.4) (2026-10-22)


### Bug Fixes

* verify cached auth token with QueryServerState ([#44](https://github.com/oliverziegert/satisfactory-operator/issues/44)) ([6159a1a](https://github.com/oliverziegert/satisfactory-operator/commit/6159a1a66023dbe5be08c851d81e168422eff344))

## [0.1.3](https://github.com/oliverziegert/satisfactory-operator/compare/v0.1.2...v0.1.3) (2026-10-22)


### Bug Fixes

* pin NodePort ports to the game ports and reconcile Service ports ([#39](https://github.com/oliverziegert/satisfactory-operator/issues/39)) ([9bd49c5](https://github.com/oliverziegert/satisfactory-operator/commit/9bd49c525aded6b9ded290700cdc9e230718035a))
* re-probe running servers and report unknown player count ([#42](https://github.com/oliverziegert/satisfactory-operator/issues/42)) ([9a126f3](https://github.com/oliverziegert/satisfactory-operator/commit/9a126f3d0f69b16325c687df83507ec519c1d2ab)), closes [#40](https://github.com/oliverziegert/satisfactory-operator/issues/40)

## [0.1.2](https://github.com/oliverziegert/satisfactory-operator/compare/v0.1.1...v0.1.2) (2026-10-21)


### Bug Fixes

* **deps:** update golang:1.27 docker digest to f20d83a ([#35](https://github.com/oliverziegert/satisfactory-operator/issues/35)) ([2ece40d](https://github.com/oliverziegert/satisfactory-operator/commit/2ece40d69ead14278517a0dfdc4866c807906d4e))
* readiness probe, claim detection and API error handling ([#36](https://github.com/oliverziegert/satisfactory-operator/issues/36)) ([cd5bb5b](https://github.com/oliverziegert/satisfactory-operator/commit/cd5bb5b72993e3d449fc9b4fa265b6d523298f8f))
* report a crash-looping server pod as `Degraded` ([#37](https://github.com/oliverziegert/satisfactory-operator/issues/37)) ([8380e73](https://github.com/oliverziegert/satisfactory-operator/commit/8380e73f3b98570df0f2cc368322ba439fa1555e))


### Miscellaneous Chores

* archive `release-pipeline` OpenSpec change ([#33](https://github.com/oliverziegert/satisfactory-operator/issues/33)) ([afee666](https://github.com/oliverziegert/satisfactory-operator/commit/afee6664b31eda20c73ed747d31b24f18e3733f0))

## [0.1.1](https://github.com/oliverziegert/satisfactory-operator/compare/v0.1.0...v0.1.1) (2026-10-21)


### Bug Fixes

* **deploy:** stop Renovate from digest-pinning the operator image ([#32](https://github.com/oliverziegert/satisfactory-operator/issues/32)) ([545c163](https://github.com/oliverziegert/satisfactory-operator/commit/545c163e10d0739fdea4b0f90f59f9453f23cbda))
* **deps:** update gcr.io/distroless/static:nonroot docker digest to d620678 ([#31](https://github.com/oliverziegert/satisfactory-operator/issues/31)) ([4f3d8b5](https://github.com/oliverziegert/satisfactory-operator/commit/4f3d8b52af9e9bdca06147002154f3323ebd3026))
* **deps:** update golang:1.27 docker digest to 6928d94 ([#27](https://github.com/oliverziegert/satisfactory-operator/issues/27)) ([4779f26](https://github.com/oliverziegert/satisfactory-operator/commit/4779f2603ea1bbcbf73f7a54228f9d7629dfd3a7))
* **deps:** update golang:1.27 docker digest to f241642 ([#28](https://github.com/oliverziegert/satisfactory-operator/issues/28)) ([1451f07](https://github.com/oliverziegert/satisfactory-operator/commit/1451f076997e92b2bdcfeb1976751441b678ad6a))


### Miscellaneous Chores

* **deps:** pin ghcr.io/oliverziegert/satisfactory-operator docker tag to bcdd211 ([#25](https://github.com/oliverziegert/satisfactory-operator/issues/25)) ([d1aa500](https://github.com/oliverziegert/satisfactory-operator/commit/d1aa5006f067ae4b677a12e17a58075400256be8))
* **deps:** update ghcr.io/oliverziegert/satisfactory-operator:v0.1.0 docker digest to 4b498cb ([#30](https://github.com/oliverziegert/satisfactory-operator/issues/30)) ([68b0671](https://github.com/oliverziegert/satisfactory-operator/commit/68b0671ba42c2dc189b699cdefe55126a10b0234))
* drop `release-as` now that v0.1.0 is released ([#29](https://github.com/oliverziegert/satisfactory-operator/issues/29)) ([1e69179](https://github.com/oliverziegert/satisfactory-operator/commit/1e691794475249e7f5d12511f92df29b5dcae389))

## 0.1.0 (2026-10-20)


### Features

* add release pipeline with release-please and multi-arch GHCR images ([#23](https://github.com/oliverziegert/satisfactory-operator/issues/23)) ([9a6883c](https://github.com/oliverziegert/satisfactory-operator/commit/9a6883c65a641713d0ac0b3a48aca74569523373))


### Bug Fixes

* **build:** bump `go` directive to 1.27 and let Renovate update it ([#22](https://github.com/oliverziegert/satisfactory-operator/issues/22)) ([1a3fb55](https://github.com/oliverziegert/satisfactory-operator/commit/1a3fb558440c3a13252432375e1c49774ead6f6a))
* **controller:** gate server probes behind a startup probe ([e8272c9](https://github.com/oliverziegert/satisfactory-operator/commit/e8272c9140e2dd9b49feac1a184245a8706e206b))
* **manager:** use pc-ziegert.dev domain for leader election ID ([642203e](https://github.com/oliverziegert/satisfactory-operator/commit/642203e06e10273ba1da21f4ba09043f2441292a))
