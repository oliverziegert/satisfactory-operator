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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PVCMountRoot is where the backup Job mounts pvc destinations, at <root>/<destination name>.
const PVCMountRoot = "/backup"

// nameTimeLayout is the timestamp suffix of backup names (<saveName>_<YYYYMMDD-HHMMSS>).
const nameTimeLayout = "20060102-150405"

const (
	savExt    = ".sav"
	tmpExt    = ".sav.tmp"
	staleTemp = 24 * time.Hour
)

// PVC stores saves as <Dir>/<name>.sav on a mounted volume.
type PVC struct {
	Dir string

	afterWrite func(path string) // test hook, runs before the read-back check
}

func (p *PVC) path(name, ext string) (string, error) {
	// The name contains the unvalidated spec.saveName; keep it inside Dir.
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("invalid save name %q", name)
	}
	return filepath.Join(p.Dir, name+ext), nil
}

// Put writes r to <name>.sav.tmp, syncs it, checks the read-back SHA-256 and renames it to
// <name>.sav, so a partial file is never visible under the final name.
func (p *PVC) Put(_ context.Context, name string, r io.Reader) error {
	final, err := p.path(name, savExt)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	tmp := final[:len(final)-len(savExt)] + tmpExt
	if err := p.writeVerified(tmp, r); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDir(p.Dir)
}

func (p *PVC) writeVerified(tmp string, r io.Reader) error {
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), r)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if p.afterWrite != nil {
		p.afterWrite(tmp)
	}
	got, err := fileSHA256(tmp)
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	if want := h.Sum(nil); !bytes.Equal(got, want) {
		return fmt.Errorf("checksum mismatch after write: got %x, want %x", got, want)
	}
	return nil
}

// List returns the <prefix>*.sav files, sorted by name, and removes .tmp files older than 24h.
// ModTime comes from the name's timestamp, falling back to the file's mtime.
func (p *PVC) List(_ context.Context, prefix string) ([]Object, error) {
	entries, err := os.ReadDir(p.Dir)
	if err != nil {
		return nil, err
	}
	var objs []Object
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		file := e.Name()
		if strings.HasSuffix(file, tmpExt) {
			if time.Since(info.ModTime()) > staleTemp {
				_ = os.Remove(filepath.Join(p.Dir, file)) // a crashed Job's leftover; retried next run
			}
			continue
		}
		name, ok := strings.CutSuffix(file, savExt)
		if !ok || !strings.HasPrefix(name, prefix) {
			continue
		}
		mod := info.ModTime()
		if i := strings.LastIndexByte(name, '_'); i >= 0 {
			if t, err := time.Parse(nameTimeLayout, name[i+1:]); err == nil {
				mod = t
			}
		}
		objs = append(objs, Object{Name: name, Size: info.Size(), ModTime: mod})
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Name < objs[j].Name })
	return objs, nil
}

// Delete removes <name>.sav.
func (p *PVC) Delete(_ context.Context, name string) error {
	path, err := p.path(name, savExt)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// Get opens <name>.sav.
func (p *PVC) Get(_ context.Context, name string) (io.ReadCloser, error) {
	path, err := p.path(name, savExt)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func fileSHA256(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	return err
}
