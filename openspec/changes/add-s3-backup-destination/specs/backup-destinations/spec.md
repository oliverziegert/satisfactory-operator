# Spec Delta

## ADDED Requirements

### Requirement: S3 destination
The `s3` destination type SHALL upload saves as `<prefix>/<saveName>_<timestamp>.sav` objects into an existing bucket. It SHALL support AWS S3 and S3-compatible stores through an optional `endpoint`, an optional `region` (default `us-east-1`) and an optional `forcePathStyle`. Uploads SHALL send a SHA-256 checksum. Uploads of any size SHALL work without the size being known in advance.

#### Scenario: Upload to an S3-compatible store
- **WHEN** a run has destination `{name: offsite, s3: {bucket: backups, prefix: factory, endpoint: https://rustfs.example, forcePathStyle: true, credentialsSecretRef: {name: s3-creds}}}`
- **THEN** the bucket `backups` contains `factory/<saveName>_<timestamp>.sav`, and the destination reports success

#### Scenario: Retention on the bucket
- **WHEN** the effective `maxCount` for `offsite` is 30 and the prefix holds 31 objects for the session
- **THEN** the oldest object is deleted, and objects not starting with `<prefix>/<saveName>_` are untouched

#### Scenario: Bucket missing
- **WHEN** the bucket doesn't exist
- **THEN** the destination reports failure with an error containing `NoSuchBucket`, and the server copy stays

### Requirement: S3 credentials
When `credentialsSecretRef` is set, the destination SHALL authenticate with the `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` keys of that Secret in the SaveGame's namespace, independently of other `s3` destinations. When it is not set, the destination SHALL use the SDK's default credential chain. Credential values SHALL NOT appear in status, Events or logs.

#### Scenario: Two buckets with different credentials
- **WHEN** a SaveGame has two `s3` destinations referencing the Secrets `a` and `b`
- **THEN** each upload is authenticated with its own Secret's keys

#### Scenario: Wrong credentials
- **WHEN** the store rejects the keys
- **THEN** the destination reports failure with an error such as `InvalidAccessKeyId` or `AccessDenied`, and no credential value appears in it

### Requirement: S3 credentials check
Before creating a backup Job, the operator SHALL verify that each Secret referenced by an `s3` destination exists and contains `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` with non-empty values. If not, it SHALL create no Job and SHALL set `Ready=False` with reason `CredentialsSecretInvalid`, naming the Secret and the missing key.

#### Scenario: Missing key
- **WHEN** the referenced Secret has no `AWS_SECRET_ACCESS_KEY`
- **THEN** no Job is created and the SaveGame shows `Ready=False` with reason `CredentialsSecretInvalid`, naming the missing key
