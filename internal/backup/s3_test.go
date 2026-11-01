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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

// s3Stub is a path-style, single-bucket S3 with two keys per ListObjectsV2 page.
type s3Stub struct {
	mu      sync.Mutex
	objects map[string][]byte
	keyID   string
}

func (s *s3Stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fail := func(status int, code string) {
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, "<Error><Code>%s</Code><Message>%s message</Message></Error>", code, code)
	}
	if !strings.Contains(r.Header.Get("Authorization"), "Credential="+s.keyID+"/") {
		fail(http.StatusForbidden, "InvalidAccessKeyId")
		return
	}
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if bucket != "backups" {
		fail(http.StatusNotFound, "NoSuchBucket")
		return
	}
	switch {
	case r.Method == http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		sum := sha256.Sum256(body)
		if r.Header.Get("X-Amz-Checksum-Sha256") != base64.StdEncoding.EncodeToString(sum[:]) {
			fail(http.StatusBadRequest, "BadDigest")
			return
		}
		s.objects[key] = body
	case r.Method == http.MethodGet && key == "":
		var keys []string
		for k := range s.objects {
			if strings.HasPrefix(k, r.URL.Query().Get("prefix")) && k > r.URL.Query().Get("continuation-token") {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		next := ""
		if len(keys) > 2 {
			keys, next = keys[:2], keys[1]
		}
		_, _ = fmt.Fprint(w, `<ListBucketResult>`)
		for _, k := range keys {
			_, _ = fmt.Fprintf(w, `<Contents><Key>%s</Key><Size>%d</Size><LastModified>2026-01-01T00:00:00Z</LastModified></Contents>`,
				k, len(s.objects[k]))
		}
		if next != "" {
			_, _ = fmt.Fprintf(w, `<IsTruncated>true</IsTruncated><NextContinuationToken>%s</NextContinuationToken>`, next)
		}
		_, _ = fmt.Fprint(w, `</ListBucketResult>`)
	case r.Method == http.MethodGet:
		b, ok := s.objects[key]
		if !ok {
			fail(http.StatusNotFound, "NoSuchKey")
			return
		}
		_, _ = w.Write(b)
	case r.Method == http.MethodDelete:
		delete(s.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		fail(http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

func newTestS3(t *testing.T, bucket, prefix, keyID, secret string) (*S3, *s3Stub) {
	t.Helper()
	stub := &s3Stub{objects: map[string][]byte{}, keyID: "AKIDTEST"}
	srv := httptest.NewServer(stub)
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	for k, v := range map[string]string{S3AccessKeyIDKey: keyID, S3SecretAccessKeyKey: secret + "\n"} {
		if err := os.WriteFile(filepath.Join(dir, k), []byte(v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d, err := NewS3(context.Background(), &satisfactoryv1alpha1.S3Destination{
		Bucket: bucket, Prefix: prefix, Endpoint: srv.URL, ForcePathStyle: true,
	}, dir)
	if err != nil {
		t.Fatal(err)
	}
	return d, stub
}

func TestS3PutListDeleteGet(t *testing.T) {
	ctx := context.Background()
	d, stub := newTestS3(t, "backups", "factory/", "AKIDTEST", "s3cr3t")
	stub.objects["factory/other_20260101-000000.sav"] = []byte("x")
	stub.objects["factory/save_notes.txt"] = []byte("x")
	stub.objects["save_20260101-000000.sav"] = []byte("x") // outside the prefix

	names := []string{"save_20261101-120000", "save_20261101-130000", "save_20261101-140000"}
	for _, n := range names {
		if err := d.Put(ctx, n, bytes.NewReader([]byte("data "+n))); err != nil {
			t.Fatalf("put %s: %v", n, err)
		}
	}
	if _, ok := stub.objects["factory/save_20261101-120000.sav"]; !ok {
		t.Fatalf("unexpected keys %v", slices.Collect(maps.Keys(stub.objects)))
	}

	objs, err := d.List(ctx, "save_")
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 3 || objs[0].Name != names[0] || objs[2].Name != names[2] ||
		!objs[0].ModTime.Equal(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)) || objs[0].Size != int64(len("data "+names[0])) {
		t.Fatalf("unexpected list %+v", objs)
	}

	one := int32(1)
	kept, err := applyRetention(ctx, d, objs, &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: &one}, time.Now())
	if err != nil || len(kept) != 1 || kept[0].Name != names[2] {
		t.Fatalf("retention kept %+v, err %v", kept, err)
	}
	for _, k := range []string{"factory/other_20260101-000000.sav", "factory/save_notes.txt", "save_20260101-000000.sav"} {
		if _, ok := stub.objects[k]; !ok {
			t.Errorf("retention touched %s", k)
		}
	}

	rc, err := d.Get(ctx, names[2])
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "data "+names[2] {
		t.Errorf("get returned %q", b)
	}
	if err := d.Put(ctx, "../x", bytes.NewReader(nil)); err == nil {
		t.Error("put accepted a name with a path separator")
	}
}

func TestS3Errors(t *testing.T) {
	ctx := context.Background()
	d, _ := newTestS3(t, "missing", "", "AKIDTEST", "s3cr3t")
	if err := d.Put(ctx, "save_20261101-120000", bytes.NewReader([]byte("x"))); err == nil ||
		!strings.HasPrefix(err.Error(), "NoSuchBucket: ") {
		t.Errorf("missing bucket: %v", err)
	}

	d, _ = newTestS3(t, "backups", "", "AKIDWRONG", "topsecretvalue")
	err := d.Put(ctx, "save_20261101-120000", bytes.NewReader([]byte("x")))
	if err == nil || !strings.HasPrefix(err.Error(), "InvalidAccessKeyId: ") || strings.Contains(err.Error(), "topsecretvalue") {
		t.Errorf("wrong credentials: %v", err)
	}
	if _, err := d.List(ctx, "save_"); err == nil || strings.Contains(err.Error(), "topsecretvalue") {
		t.Errorf("list with wrong credentials: %v", err)
	}
}
