## 1. CRD Type Updates

- [x] 1.1 Add `ServiceType corev1.ServiceType` field to `SatisfactoryServerSpec` with kubebuilder default `"LoadBalancer"`
- [x] 1.2 Add `RetainPVCOnDelete *bool` field to `SatisfactoryServerSpec` with kubebuilder default `true`

## 2. Helper Functions

- [x] 2.1 Add `buildStatefulSet(server *v1alpha1.SatisfactoryServer) *appsv1.StatefulSet` to `satisfactoryserver_helpers.go` — 1 replica, labels, `ServiceName: <name>-headless`, pod template from `buildPodSpec`, volumeClaimTemplate from `buildPVCSpec`, ownerReference
- [x] 2.2 Add `buildHeadlessService(server *v1alpha1.SatisfactoryServer) *corev1.Service` — name `<name>-headless`, `clusterIP: None`, ports from `buildContainerPorts`, ownerReference
- [x] 2.3 Add `buildExternalService(server *v1alpha1.SatisfactoryServer) *corev1.Service` — name `<name>`, type from `spec.ServiceType`, ports from `buildContainerPorts`, ownerReference

## 3. Controller Implementation

- [x] 3.1 Add finalizer constant `satisfactoryServerFinalizer = "satisfactory.oliverziegert.github.com/finalizer"` to `satisfactoryserver_controller.go`
- [x] 3.2 Update RBAC markers on the controller to include `apps` StatefulSets and `core` Services (get, list, watch, create, update, patch, delete)
- [x] 3.3 Implement deletion handler: if `RetainPVCOnDelete == false` delete PVC `config-<name>-0`, then remove finalizer and update
- [x] 3.4 Implement normal reconciliation path: add finalizer if missing, then reconcile StatefulSet (create or update)
- [x] 3.5 Reconcile headless Service: get by name, create if not found
- [x] 3.6 Reconcile external Service: get by name, create if not found, update type if changed
- [x] 3.7 Update `status.Phase`: fetch StatefulSet ready replicas — `0` → `Pending`, `≥1` → `Running`

## 4. Verification

- [x] 4.1 Run `make generate` to regenerate DeepCopy after type changes
- [x] 4.2 Run `make manifests` to regenerate CRD and RBAC manifests
- [x] 4.3 Run `make build` to verify compilation
- [x] 4.4 Add unit tests for `buildStatefulSet` verifying replica count, labels, ServiceName, pod template image, and volumeClaimTemplate
- [x] 4.5 Add unit tests for `buildHeadlessService` verifying name, ClusterIP=None, and port definitions
- [x] 4.6 Add unit tests for `buildExternalService` verifying name, service type propagation, and ports
- [x] 4.7 Update `config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml` with a minimal working example