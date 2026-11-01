# Design

## Context

`add-backup-job` provides the `Destination` interface and the Job. The Job downloads the save once into a local file, and `Put` receives a reader on that file. The Job pod has no Kubernetes API token and runs under the namespace's `default` ServiceAccount.

## Goals / Non-Goals

**Goals:**
- One code path for AWS and S3-compatible stores.
- Several `s3` destinations with different credentials in one SaveGame.

**Non-Goals:**
- Server-side encryption settings, storage classes, object tags, Object Lock.
- A configurable Job `serviceAccountName` for IRSA or Pod Identity (follow-up if needed).
- Bucket creation. The bucket must already exist.

## Decisions

- **`aws-sdk-go-v2`, not `minio-go`.** The user decided this: they no longer trust the MinIO project after its license change, and RustFS (the user's store) recommends the AWS SDK. Configuration:
  - `config.LoadDefaultConfig` with `WithRegion(region or "us-east-1")`
  - `s3.Options.BaseEndpoint = endpoint` (if set) and `UsePathStyle = forcePathStyle`
- **Credentials as files, one directory per destination.** Each referenced Secret is mounted at `/var/run/backup/s3/<destination name>/`, and the Job reads `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` into `credentials.NewStaticCredentialsProvider`. Env vars would collide when two `s3` destinations use different Secrets. If `credentialsSecretRef` is missing, nothing is mounted and the default chain applies. Without a ServiceAccount configuration that only reaches the node's instance profile. That gets documented.
- **Secret validation in the reconciler.** It does a `Get` on the Secret and checks both keys are present and non-empty. If not, it sets `Ready=False` with reason `CredentialsSecretInvalid`, naming the Secret and the missing key, and creates no Job. The operator already has read access to Secrets. It reads the values only to check that they're there and never logs them.
- **Upload with a single `PutObject`.** The Job already holds the save in a local file, so the size is known and the body is seekable. `ChecksumAlgorithm: types.ChecksumAlgorithmSha256` lets the store verify the bytes. A single `PutObject` caps an object at 5 GiB, far above real save sizes, and leaves no multipart parts to clean up. `feature/s3/manager` was rejected because it is deprecated, and its successor `feature/s3/transfermanager` is still pre-1.0.
- **Object layout and listing.**
  - Keys are `<prefix>/<name>.sav`, with an empty prefix meaning the bucket root. A trailing `/` on `prefix` is normalized away.
  - `List` uses a `ListObjectsV2` paginator on `<prefix>/<saveName>_`, with `ModTime` parsed from the name and `LastModified` as fallback.
  - `Delete` uses `DeleteObject`, and `Get` uses `GetObject`.
- **Endpoint errors become readable status.** SDK errors are reduced to their `smithy.APIError` code and message (for example `NoSuchBucket`, `AccessDenied`, `InvalidAccessKeyId`) before going into the 256-byte error field.

## Risks / Trade-offs

- [Some S3-compatible stores reject the SDK's default request checksums or the SHA-256 checksum header] → The e2e suite runs against RustFS. If a common store rejects it, set `RequestChecksumCalculation: WhenRequired` and verify the size with `HeadObject` instead. That's an implementation detail with no new API field.
- [Bigger dependency tree] → Only the modules listed in the proposal are used. They're pure Go and add to the manager binary only, which already contains the `backup` subcommand.
- [Credentials stored in a namespaced Secret] → That's standard Kubernetes practice. The Job mounts the Secret read-only, and `automountServiceAccountToken: false` limits what a compromised Job can reach.
- [The default chain without IRSA is mostly useless in-cluster] → It's documented, and a `serviceAccountName` option is the follow-up.
