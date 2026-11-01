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

// Package backup runs a backup inside the backup Job: save, download once, write to every
// destination and apply each destination's retention.
package backup

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// saveDateTimeLayout is the format of SaveHeader.SaveDateTime (e.g. "2024.09.28-04.00.32").
const saveDateTimeLayout = "2006.01.02-15.04.05"

// Object is one save held by a destination.
type Object struct {
	Name    string
	Size    int64
	ModTime time.Time // zero when the destination cannot tell
}

// Destination stores saves. Names are bare save names (<saveName>_<YYYYMMDD-HHMMSS>);
// file stores add their extension internally.
type Destination interface {
	Put(ctx context.Context, name string, r io.Reader) error
	List(ctx context.Context, prefix string) ([]Object, error)
	Delete(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (io.ReadCloser, error)
}

// Resolve returns the SaveGame's destinations with the effective retention policy filled in.
// An empty list means the server destination only; the default is not written to the object.
func Resolve(spec satisfactoryv1alpha1.SaveGameSpec) []satisfactoryv1alpha1.SaveGameDestination {
	dests := spec.Destinations
	if len(dests) == 0 {
		dests = []satisfactoryv1alpha1.SaveGameDestination{{Name: "server", Server: &satisfactoryv1alpha1.ServerDestination{}}}
	}
	out := make([]satisfactoryv1alpha1.SaveGameDestination, len(dests))
	for i := range dests {
		out[i] = *dests[i].DeepCopy()
		if out[i].RetentionPolicy == nil {
			out[i].RetentionPolicy = spec.RetentionPolicy.DeepCopy()
		}
	}
	return out
}

// Target is a destination ready for a run.
type Target struct {
	Name      string
	Retention *satisfactoryv1alpha1.SaveGameRetentionPolicy
	Dest      Destination
}

// Targets opens the resolved destinations.
func Targets(dests []satisfactoryv1alpha1.SaveGameDestination, game *satisfactory.Client) ([]Target, error) {
	targets := make([]Target, 0, len(dests))
	for _, d := range dests {
		t := Target{Name: d.Name, Retention: d.RetentionPolicy}
		switch {
		case d.Server != nil:
			t.Dest = &Server{Client: game}
		default:
			return nil, fmt.Errorf("destination %q has no type this binary supports", d.Name)
		}
		targets = append(targets, t)
	}
	return targets, nil
}

// Server keeps saves on the game server.
type Server struct {
	Client *satisfactory.Client
}

// Put does nothing: the game API save already wrote the file on the server.
func (s *Server) Put(context.Context, string, io.Reader) error { return nil }

// List returns the server's saves whose name starts with prefix, sorted by name.
// The API reports no size, so Size is 0.
func (s *Server) List(ctx context.Context, prefix string) ([]Object, error) {
	resp, err := s.Client.EnumerateSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("enumerate sessions: %w", err)
	}
	var objs []Object
	for _, session := range resp.Sessions {
		for _, h := range session.SaveHeaders {
			if !strings.HasPrefix(h.SaveName, prefix) {
				continue
			}
			t, _ := time.Parse(saveDateTimeLayout, h.SaveDateTime) // zero if unparseable
			objs = append(objs, Object{Name: h.SaveName, ModTime: t})
		}
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Name < objs[j].Name })
	return objs, nil
}

// Delete deletes a save file from the server.
func (s *Server) Delete(ctx context.Context, name string) error {
	return s.Client.DeleteSaveFile(ctx, name)
}

// Get streams a save file from the server.
func (s *Server) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	return s.Client.DownloadSaveGame(ctx, name)
}

// applyRetention deletes the oldest objects beyond MaxCount and any object older than MaxAge.
// objs must be sorted oldest first (by name, since the timestamp suffix is zero-padded).
// It returns the objects that remain.
func applyRetention(
	ctx context.Context,
	d Destination,
	objs []Object,
	policy *satisfactoryv1alpha1.SaveGameRetentionPolicy,
	now time.Time,
) ([]Object, error) {
	if policy == nil {
		return objs, nil
	}
	excess := 0
	if policy.MaxCount != nil {
		excess = max(0, len(objs)-int(*policy.MaxCount))
	}
	var kept []Object
	for i, o := range objs {
		// Objects without a ModTime are kept: never delete what we cannot date.
		expired := policy.MaxAge != nil && !o.ModTime.IsZero() && now.Sub(o.ModTime) > policy.MaxAge.Duration
		if i < excess || expired {
			if err := d.Delete(ctx, o.Name); err != nil {
				return nil, fmt.Errorf("delete %s: %w", o.Name, err)
			}
			continue
		}
		kept = append(kept, o)
	}
	return kept, nil
}
