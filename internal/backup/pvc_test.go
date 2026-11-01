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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPVCDestination(t *testing.T) {
	ctx := context.Background()

	t.Run("round trip in a new subdirectory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "factory", "hourly")
		p := &PVC{Dir: dir}
		if err := p.Put(ctx, "save_20261101-120000", strings.NewReader(saveData)); err != nil {
			t.Fatalf("Put: %v", err)
		}
		objs, err := p.List(ctx, "save_")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		want := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		if len(objs) != 1 || objs[0].Name != "save_20261101-120000" || objs[0].Size != int64(len(saveData)) ||
			!objs[0].ModTime.Equal(want) {
			t.Fatalf("List = %+v", objs)
		}
		rc, err := p.Get(ctx, objs[0].Name)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(b) != saveData {
			t.Fatalf("Get = %q", b)
		}
		if err := p.Delete(ctx, objs[0].Name); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if objs, _ := p.List(ctx, "save_"); len(objs) != 0 {
			t.Fatalf("List after Delete = %+v", objs)
		}
	})

	t.Run("leftover tmp files are ignored and stale ones removed", func(t *testing.T) {
		dir := t.TempDir()
		fresh := filepath.Join(dir, "save_20261101-120000.sav.tmp")
		stale := filepath.Join(dir, "save_20261031-120000.sav.tmp")
		for _, f := range []string{fresh, stale} {
			if err := os.WriteFile(f, []byte("partial"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		old := time.Now().Add(-25 * time.Hour)
		if err := os.Chtimes(stale, old, old); err != nil {
			t.Fatal(err)
		}
		objs, err := (&PVC{Dir: dir}).List(ctx, "save_")
		if err != nil || len(objs) != 0 {
			t.Fatalf("List = %+v, %v", objs, err)
		}
		if _, err := os.Stat(fresh); err != nil {
			t.Errorf("fresh tmp removed: %v", err)
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Errorf("stale tmp kept: %v", err)
		}
	})

	t.Run("mtime is the fallback ModTime", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "save_copy.sav")
		if err := os.WriteFile(f, []byte(saveData), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		if err := os.Chtimes(f, mt, mt); err != nil {
			t.Fatal(err)
		}
		objs, err := (&PVC{Dir: dir}).List(ctx, "save_")
		if err != nil || len(objs) != 1 || !objs[0].ModTime.Equal(mt) {
			t.Fatalf("List = %+v, %v", objs, err)
		}
	})

	t.Run("hash mismatch fails and cleans up", func(t *testing.T) {
		dir := t.TempDir()
		p := &PVC{Dir: dir, afterWrite: func(path string) { _ = os.WriteFile(path, []byte("corrupt"), 0o644) }}
		err := p.Put(ctx, "save_20261101-120000", strings.NewReader(saveData))
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("Put err = %v", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Fatalf("dir not clean: %v", entries)
		}
	})

	t.Run("names cannot escape the directory", func(t *testing.T) {
		p := &PVC{Dir: t.TempDir()}
		for _, name := range []string{"../evil", "a/b", "..", ""} {
			if err := p.Put(ctx, name, strings.NewReader(saveData)); err == nil {
				t.Errorf("Put(%q) succeeded", name)
			}
		}
	})
}
