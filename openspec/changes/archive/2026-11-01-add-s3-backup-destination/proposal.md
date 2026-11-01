# Proposal

## Why

A PVC destination still lives in the same cluster, and often on the same hardware. Real off-site backups need object storage somewhere else: AWS S3, or an S3-compatible store such as RustFS, Garage, Backblaze B2, Cloudflare R2 or Hetzner. With the `Destination` interface from `add-backup-job`, this is one more implementation plus an API field.

## What Changes

- New destination type:
  ```yaml
  s3:
    bucket: satisfactory-backups
    prefix: prod/factory          # optional
    endpoint: https://s3.example.com   # optional, AWS if unset
    region: eu-central-1          # optional, default us-east-1
    forcePathStyle: true          # optional
    credentialsSecretRef:         # optional
      name: s3-creds              # keys AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
  ```
- Uses **`aws-sdk-go-v2`**: `BaseEndpoint`, `UsePathStyle`, and a single `PutObject` of the downloaded file (up to 5 GiB; `feature/s3/manager` is deprecated and its successor `transfermanager` is pre-1.0). `minio-go` was rejected (licensing and project trust), and S3-compatible stores such as RustFS recommend the AWS SDK.
- Objects are named `<prefix>/<saveName>_<ts>.sav` and uploaded with `ChecksumAlgorithm: SHA256`, so the store verifies integrity on upload.
- Credentials: the referenced Secret is mounted read-only into the Job for each destination, and the SDK uses a static credentials provider. Without `credentialsSecretRef`, the SDK's default credential chain is used.
- Before creating a Job, the reconciler checks that each referenced Secret exists and has both keys. Otherwise it sets `Ready=False` with reason `CredentialsSecretInvalid`.

## Capabilities

### Modified Capabilities
- `backup-destinations`: adds the `s3` destination type.

## Impact

- `go.mod`: `github.com/aws/aws-sdk-go-v2` (`config`, `credentials`, `service/s3`, plus `smithy-go` for error codes). Renovate's `gomod` rules cover it, and its updates release as `fix(deps)`.
- `api/v1alpha1/savegame_types.go`: `S3 *S3Destination`. Run `make manifests generate helm`.
- `internal/backup/`: the `s3` destination.
- `internal/controller/`: a Secret volume per `s3` destination and the Secret check. RBAC for reading Secrets already exists.
- README: the `s3` example, the Secret format and the credential chain notes.
- Depends on `add-backup-job`. Independent of `add-pvc-backup-destination`.
