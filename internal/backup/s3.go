/*
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
*/

package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

// S3CredentialsRoot is where the backup Job mounts s3 credentials Secrets, at <root>/<destination name>.
const S3CredentialsRoot = "/var/run/backup/s3"

// S3 credentials Secret keys.
const (
	S3AccessKeyIDKey     = "AWS_ACCESS_KEY_ID"
	S3SecretAccessKeyKey = "AWS_SECRET_ACCESS_KEY"
)

// S3 stores saves as <Prefix>/<name>.sav objects in Bucket.
type S3 struct {
	Client *s3.Client
	Bucket string
	Prefix string // without a trailing "/"; empty means the bucket root
}

// NewS3 builds the destination. credsDir holds the credentials Secret's files; empty means
// the SDK's default credential chain.
func NewS3(ctx context.Context, d *satisfactoryv1alpha1.S3Destination, credsDir string) (*S3, error) {
	region := d.Region
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if credsDir != "" {
		id, err := readCredential(credsDir, S3AccessKeyIDKey)
		if err != nil {
			return nil, err
		}
		secret, err := readCredential(credsDir, S3SecretAccessKeyKey)
		if err != nil {
			return nil, err
		}
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(id, secret, "")))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if d.Endpoint != "" {
			o.BaseEndpoint = aws.String(d.Endpoint)
		}
		o.UsePathStyle = d.ForcePathStyle
	})
	return &S3{Client: client, Bucket: d.Bucket, Prefix: strings.TrimRight(d.Prefix, "/")}, nil
}

func readCredential(dir, key string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil {
		return "", fmt.Errorf("read credential %s: %w", key, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func (s *S3) keyPrefix() string {
	if s.Prefix == "" {
		return ""
	}
	return s.Prefix + "/"
}

func (s *S3) key(name string) (string, error) {
	// The name contains the unvalidated spec.saveName; keep it inside the prefix.
	if name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid save name %q", name)
	}
	return s.keyPrefix() + name + savExt, nil
}

// Put uploads r as one PutObject with a SHA-256 checksum. r should be seekable (the Job
// passes the downloaded file), so the SDK can compute the length and checksum.
// shortcut: a single PutObject caps a save at 5 GiB, switch to multipart if saves get that big.
func (s *S3) Put(ctx context.Context, name string, r io.Reader) error {
	key, err := s.key(name)
	if err != nil {
		return err
	}
	_, err = s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(s.Bucket),
		Key:               aws.String(key),
		Body:              r,
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
	})
	return s3Error(err)
}

// List returns the <Prefix>/<prefix>*.sav objects, sorted by name. ModTime comes from the
// name's timestamp, falling back to LastModified.
func (s *S3) List(ctx context.Context, prefix string) ([]Object, error) {
	kp := s.keyPrefix()
	p := s3.NewListObjectsV2Paginator(s.Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.Bucket),
		Prefix: aws.String(kp + prefix),
	})
	var objs []Object
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, s3Error(err)
		}
		for _, o := range page.Contents {
			name, ok := strings.CutSuffix(strings.TrimPrefix(aws.ToString(o.Key), kp), savExt)
			if !ok || strings.Contains(name, "/") {
				continue
			}
			mod := aws.ToTime(o.LastModified)
			if i := strings.LastIndexByte(name, '_'); i >= 0 {
				if t, err := time.Parse(nameTimeLayout, name[i+1:]); err == nil {
					mod = t
				}
			}
			objs = append(objs, Object{Name: name, Size: aws.ToInt64(o.Size), ModTime: mod})
		}
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Name < objs[j].Name })
	return objs, nil
}

// Delete removes <name>.sav.
func (s *S3) Delete(ctx context.Context, name string) error {
	key, err := s.key(name)
	if err != nil {
		return err
	}
	_, err = s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	return s3Error(err)
}

// Get streams <name>.sav.
func (s *S3) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	key, err := s.key(name)
	if err != nil {
		return nil, err
	}
	out, err := s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	if err != nil {
		return nil, s3Error(err)
	}
	return out.Body, nil
}

// s3Error reduces SDK errors to the store's code and message, which fit the 256-byte status field.
func s3Error(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%s: %s", apiErr.ErrorCode(), apiErr.ErrorMessage())
	}
	return err
}
