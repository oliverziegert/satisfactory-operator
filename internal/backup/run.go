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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// maxErrorLen keeps the termination message below its 4 KiB limit.
const maxErrorLen = 256

const putAttempts = 3

// putBackoff is the wait before the second Put attempt; it doubles for each later attempt.
var putBackoff = 2 * time.Second

// Result is the JSON a backup Job writes to its termination message.
type Result struct {
	SaveName     string              `json:"saveName"`
	Saved        bool                `json:"saved"`
	Size         int64               `json:"size,omitempty"`
	SHA256       string              `json:"sha256,omitempty"`
	Error        string              `json:"error,omitempty"`
	Destinations []DestinationResult `json:"destinations"`
}

// DestinationResult is the outcome for one destination.
type DestinationResult struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Retained int32  `json:"retained,omitempty"`
	Error    string `json:"error,omitempty"`
}

// OK reports whether the save and every destination succeeded.
func (r Result) OK() bool {
	return r.Saved && r.Error == "" &&
		!slices.ContainsFunc(r.Destinations, func(d DestinationResult) bool { return !d.OK })
}

// Run is one backup run.
type Run struct {
	Game     *satisfactory.Client
	SaveName string // <prefix>_<YYYYMMDD-HHMMSS>, picked by the reconciler
	Prefix   string // spec.saveName; retention only considers <Prefix>_*
	Targets  []Target
	Dir      string // scratch directory for the downloaded save
	Now      time.Time
}

// Do saves, downloads the save once, writes it to every target and applies retention.
// Without a server target it removes server copies that every target holds.
func (r Run) Do(ctx context.Context) Result {
	res := Result{SaveName: r.SaveName, Destinations: make([]DestinationResult, len(r.Targets))}
	for i, t := range r.Targets {
		res.Destinations[i].Name = t.Name
	}
	fail := func(err error) Result {
		res.Error = truncate(err.Error())
		for i := range res.Destinations {
			res.Destinations[i].Error = truncate("skipped: " + res.Error)
		}
		return res
	}

	if err := r.Game.SaveGame(ctx, r.SaveName); err != nil {
		return fail(fmt.Errorf("save game %s: %w", r.SaveName, err))
	}
	res.Saved = true

	path, size, sum, err := r.download(ctx)
	if err != nil {
		return fail(fmt.Errorf("download %s: %w", r.SaveName, err))
	}
	res.Size, res.SHA256 = size, sum

	held := make([][]Object, len(r.Targets))
	allOK, hasServer := true, false
	for i, t := range r.Targets {
		if _, ok := t.Dest.(*Server); ok {
			hasServer = true
		}
		kept, err := r.write(ctx, t, path)
		if err != nil {
			res.Destinations[i].Error = truncate(err.Error())
			allOK = false
			continue
		}
		res.Destinations[i].OK = true
		res.Destinations[i].Retained = int32(len(kept))
		held[i] = kept
	}
	if !hasServer && allOK {
		r.removeServerCopies(ctx, held)
	}
	return res
}

// download streams the save into Dir, returning its path, size and SHA-256.
func (r Run) download(ctx context.Context) (string, int64, string, error) {
	rc, err := r.Game.DownloadSaveGame(ctx, r.SaveName)
	if err != nil {
		return "", 0, "", err
	}
	defer func() { _ = rc.Close() }()

	// A fixed file name keeps the user-chosen save name out of the path.
	path := filepath.Join(r.Dir, "save.sav")
	f, err := os.Create(path)
	if err != nil {
		return "", 0, "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), rc)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", 0, "", err
	}
	return path, n, hex.EncodeToString(h.Sum(nil)), nil
}

// write puts the file to t, then applies t's retention. It returns what t holds afterwards.
func (r Run) write(ctx context.Context, t Target, path string) ([]Object, error) {
	if err := put(ctx, t.Dest, r.SaveName, path); err != nil {
		return nil, fmt.Errorf("put: %w", err)
	}
	objs, err := t.Dest.List(ctx, r.Prefix+"_")
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	kept, err := applyRetention(ctx, t.Dest, objs, t.Retention, r.Now)
	if err != nil {
		return nil, fmt.Errorf("retention: %w", err)
	}
	return kept, nil
}

// put tries Put up to putAttempts times with exponential backoff, reopening the file each time.
func put(ctx context.Context, d Destination, name, path string) error {
	var err error
	for attempt := range putAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(putBackoff << (attempt - 1)):
			}
		}
		if err = putFile(ctx, d, name, path); err == nil {
			return nil
		}
	}
	return err
}

func putFile(ctx context.Context, d Destination, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return d.Put(ctx, name, f)
}

// removeServerCopies deletes this run's save from the game server, plus older <Prefix>_*
// saves that every target holds (leftovers of earlier runs). Only call it when every
// target succeeded and none of them is the server.
func (r Run) removeServerCopies(ctx context.Context, held [][]Object) {
	server := &Server{Client: r.Game}
	objs, err := server.List(ctx, r.Prefix+"_")
	if err != nil {
		// Not a run failure: the copies stay and the next successful run retries.
		log.Printf("list server saves for cleanup: %v", err)
		return
	}
	for _, o := range objs {
		if o.Name != r.SaveName && !heldByAll(o.Name, held) {
			continue
		}
		if err := server.Delete(ctx, o.Name); err != nil {
			log.Printf("delete server copy %s: %v", o.Name, err)
		}
	}
}

func heldByAll(name string, held [][]Object) bool {
	for _, objs := range held {
		if !slices.ContainsFunc(objs, func(o Object) bool { return o.Name == name }) {
			return false
		}
	}
	return true
}

func truncate(s string) string {
	if len(s) <= maxErrorLen {
		return s
	}
	return strings.ToValidUTF8(s[:maxErrorLen], "")
}
