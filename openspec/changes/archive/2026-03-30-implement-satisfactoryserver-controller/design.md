## Context

The project has three fully-defined CRDs, a complete Satisfactory API client (`pkg/satisfactory`), and Pod/PVC builder helpers (`buildPodSpec`, `buildPVCSpec`, `buildContainerPorts`). All three controllers are stubs. This design covers the first real controller: `SatisfactoryServer`. The goal is to make a `SatisfactoryServer` CR actually deploy and manage a running server on Kubernetes.

## Goals / Non-Goals

**Goals:**
- Create a StatefulSet, headless Service, and external Service for each `SatisfactoryServer` resource
- Protect save data with a finalizer and configurable PVC retention on delete
- Report basic lifecycle phase (Pending → Running) based on StatefulSet readiness
- Add `serviceType` and `retainPVCOnDelete` to the CRD spec

**Non-Goals:**
- Status conditions (Available, Progressing, Degraded) — milestone B
- API client integration (connectedPlayers, serverVersion) — milestone C
- SaveGame or ClusterPolicy controllers
- Webhooks or admission validation

## Decisions

### 1. StatefulSet over bare Pod

**Decision:** Use a `StatefulSet` with 1 replica.

**Rationale:** Satisfactory Dedicated Server is an inherently stateful workload with a large PVC (`/config`). StatefulSets provide stable pod identity (needed for the headless Service DNS), built-in PVC lifecycle management via `volumeClaimTemplates`, and self-healing on pod deletion. Bare Pods would require the controller to re-create them on deletion — unnecessary complexity.

**Alternatives considered:**
- Bare Pod: simpler but no self-healing; controller must watch and re-create
- Deployment: no stable pod identity, PVC attachment is awkward for stateful workloads

### 2. StatefulSet volumeClaimTemplates for PVC management

**Decision:** PVC is declared in `StatefulSet.spec.volumeClaimTemplates`, not created separately by the controller.

**Rationale:** Kubernetes automatically creates the PVC (`config-<name>-0`) when the StatefulSet is created, and crucially does NOT delete it when the StatefulSet is deleted. This gives us free PVC retention by default. When `RetainPVCOnDelete=false`, the finalizer explicitly deletes the PVC before removing itself.

**Alternatives considered:**
- Controller-managed PVC: more control but duplicates what StatefulSet already does well

### 3. Two Services: headless + external

**Decision:** Create two Services per server — `<name>-headless` (ClusterIP: None) and `<name>` (type from spec).

**Rationale:** StatefulSets require a governing headless Service for pod DNS resolution. A second external Service (LoadBalancer by default) is what players connect to. Keeping them separate follows Kubernetes conventions and allows different port/annotation configurations later.

**Alternatives considered:**
- Single LoadBalancer Service: violates StatefulSet DNS requirements
- Single headless Service only: makes the server unreachable externally without additional infrastructure

### 4. Finalizer for deletion protection

**Decision:** Add `satisfactory.oliverziegert.github.com/finalizer` to every `SatisfactoryServer`.

**Rationale:** Without a finalizer, deleting the `SatisfactoryServer` resource would delete the StatefulSet immediately, but the PVC (managed by StatefulSet volumeClaimTemplates) would persist orphaned with no way to control it. The finalizer gives the controller control over the deletion sequence: optionally delete the PVC, then remove the finalizer.

### 5. Phase-only status in this milestone

**Decision:** Only update `Phase` (Pending/Running), not `Conditions`.

**Rationale:** Phase gives operators and users actionable signal (is my server up?) without the complexity of condition management. Conditions (Available, Progressing, Degraded) require careful state machine design and will be added in milestone B alongside readiness probe integration.

### 6. serviceType field default: LoadBalancer

**Decision:** `serviceType` defaults to `LoadBalancer` via kubebuilder marker.

**Rationale:** LoadBalancer is the most widely supported way to expose game servers externally across cloud providers and works out of the box with tools like MetalLB for bare metal. Users who need NodePort or ClusterIP can override the field.

## Risks / Trade-offs

- **[StatefulSet update strategy]** Changing the StatefulSet spec (e.g. image) triggers a rolling update that briefly interrupts the game server. This is acceptable for now but should be documented. → Mitigation: `RollingUpdate` is the default strategy; `OnDelete` can be added as a future field.
- **[PVC naming dependency]** The finalizer deletes PVC `config-<name>-0` by convention (StatefulSet volumeClaimTemplate name `config` + pod ordinal `0`). If the template name changes, the deletion logic breaks. → Mitigation: Keep the PVC template name constant (`config`); document this as a fixed convention.
- **[LoadBalancer provisioning]** On clusters without a cloud load balancer controller (e.g. bare metal without MetalLB), the external Service stays in `<pending>` state. → Mitigation: Document this; users should set `serviceType: NodePort` in those environments.