# Tasks

## 1. API

- [x] 1.1 Add `S3 *S3Destination` (`bucket` required, `prefix`, `endpoint` (URL format), `region`, `forcePathStyle`, `credentialsSecretRef` with `name`) to `SaveGameDestination`, and include it in the "exactly one type" CEL rule. Run `make manifests generate helm`. Verify with envtest cases: valid, missing bucket, `s3` combined with `pvc` in one entry.

## 2. Destination

- [x] 2.1 Add the `aws-sdk-go-v2` modules (`config`, `credentials`, `service/s3`, `smithy-go`) and run `go mod tidy`.
- [x] 2.2 Implement the `s3` destination: client setup (region default, `BaseEndpoint`, `UsePathStyle`, static or default credentials from a directory), `Put` via a single `PutObject` with SHA-256, a paginated `List` on `<prefix>/<saveName>_`, `Delete`, `Get`, prefix normalization, and reduction of `smithy.APIError`. Verify with unit tests against an `httptest.Server` S3 stub (single-part `PutObject`, `ListObjectsV2`, `DeleteObject`, `GetObject`, error responses): key layout, retention listing, the error string contains no secret.

## 3. Job and controller

- [x] 3.1 Mount each referenced Secret read-only at `/var/run/backup/s3/<name>/` and pass the directory in the destination config. Verify with the Job spec unit test (two `s3` destinations, two mounts).
- [x] 3.2 Validate the Secrets before creating the Job (`CredentialsSecretInvalid`, naming the Secret and the missing key, never the values). Verify with envtest: a missing Secret, a missing key, and a valid Secret that leads to a Job.

## 4. End-to-end and docs

- [x] 4.1 e2e: deploy RustFS in the Kind cluster, create a bucket and a credentials Secret, run a SaveGame with an `s3` destination, and check that the object exists (for example with `aws s3 ls` in a one-shot pod). Confirm that the SHA-256 upload checksum is accepted, and if not, apply the `WhenRequired` fallback from the design. Run `make test-e2e`.
- [x] 4.2 README: the `s3` example for AWS and for an S3-compatible endpoint, the Secret format, the default-chain limitation without a ServiceAccount, and that the bucket must exist. Verify that the YAML applies.

## Workflow follow-up

- Open the PR titled `feat: add s3 backup destination`.
- Archive the change after merge.
