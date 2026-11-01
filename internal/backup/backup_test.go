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
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

const saveData = "save-bytes"

// fakeGame is an in-memory game server API covering the save functions.
type fakeGame struct {
	saves     []satisfactory.SaveHeader
	deleted   []string
	downloads int
	failSave  bool
}

func (f *fakeGame) client(t *testing.T) *satisfactory.Client {
	t.Helper()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Data struct {
				SaveName string `json:"saveName"`
			} `json:"data"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		name := req.Data.SaveName
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "SaveGame":
			if f.failSave {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errorCode":"save_failed"}`))
				return
			}
			f.saves = append(f.saves, satisfactory.SaveHeader{SaveName: name, SaveDateTime: "2026.11.01-12.00.00"})
			w.WriteHeader(http.StatusNoContent)
		case "DownloadSaveGame":
			f.downloads++
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(saveData))
		case "EnumerateSessions":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": satisfactory.EnumerateSessionsResponse{
				Sessions: []satisfactory.SessionSaveStruct{{SessionName: "world", SaveHeaders: f.saves}},
			}})
		case "DeleteSaveFile":
			f.deleted = append(f.deleted, name)
			f.saves = slices.DeleteFunc(f.saves, func(h satisfactory.SaveHeader) bool { return h.SaveName == name })
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(ts.Close)
	return satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // test server
}

func (f *fakeGame) names() []string {
	names := make([]string, 0, len(f.saves))
	for _, h := range f.saves {
		names = append(names, h.SaveName)
	}
	return names
}

// memDest is an in-memory Destination.
type memDest struct {
	objs    map[string]Object
	data    map[string]string
	puts    int
	failPut bool
}

func newMemDest(names ...string) *memDest {
	d := &memDest{objs: map[string]Object{}, data: map[string]string{}}
	for _, n := range names {
		d.objs[n] = Object{Name: n}
	}
	return d
}

func (d *memDest) Put(_ context.Context, name string, r io.Reader) error {
	d.puts++
	if d.failPut {
		return errors.New("disk full")
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	d.data[name] = string(b)
	d.objs[name] = Object{Name: name, Size: int64(len(b))}
	return nil
}

func (d *memDest) List(_ context.Context, prefix string) ([]Object, error) {
	var out []Object
	for _, n := range slices.Sorted(maps.Keys(d.objs)) {
		if strings.HasPrefix(n, prefix) {
			out = append(out, d.objs[n])
		}
	}
	return out, nil
}

func (d *memDest) Delete(_ context.Context, name string) error {
	delete(d.objs, name)
	return nil
}

func (d *memDest) Get(_ context.Context, name string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(d.data[name])), nil
}

func TestResolve(t *testing.T) {
	specPolicy := &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](30)}
	ownPolicy := &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](3)}
	tests := []struct {
		name      string
		spec      satisfactoryv1alpha1.SaveGameSpec
		wantNames []string
		wantMax   []int32 // 0 = no policy
	}{
		{name: "empty list defaults to server", spec: satisfactoryv1alpha1.SaveGameSpec{RetentionPolicy: specPolicy},
			wantNames: []string{"server"}, wantMax: []int32{30}},
		{name: "empty list without policy", wantNames: []string{"server"}, wantMax: []int32{0}},
		{name: "inherited and overridden retention", spec: satisfactoryv1alpha1.SaveGameSpec{
			RetentionPolicy: specPolicy,
			Destinations: []satisfactoryv1alpha1.SaveGameDestination{
				{Name: "local", RetentionPolicy: ownPolicy, Server: &satisfactoryv1alpha1.ServerDestination{}},
				{Name: "nas"},
			},
		}, wantNames: []string{"local", "nas"}, wantMax: []int32{3, 30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve(tt.spec)
			var names []string
			var maxCounts []int32
			for _, d := range got {
				names = append(names, d.Name)
				m := int32(0)
				if d.RetentionPolicy != nil {
					m = *d.RetentionPolicy.MaxCount
				}
				maxCounts = append(maxCounts, m)
			}
			if !slices.Equal(names, tt.wantNames) || !slices.Equal(maxCounts, tt.wantMax) {
				t.Errorf("got %v %v, want %v %v", names, maxCounts, tt.wantNames, tt.wantMax)
			}
		})
	}
}

func TestServerDestination(t *testing.T) {
	game := &fakeGame{saves: []satisfactory.SaveHeader{
		{SaveName: "f_20261101-120000", SaveDateTime: "2026.11.01-12.00.00"},
		{SaveName: "f_20261031-120000", SaveDateTime: "garbage"},
		{SaveName: "other_20261101-120000", SaveDateTime: "2026.11.01-12.00.00"},
		{SaveName: "f", SaveDateTime: "2026.11.01-12.00.00"},
	}}
	s := &Server{Client: game.client(t)}
	ctx := context.Background()

	objs, err := s.List(ctx, "f_")
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 || objs[0].Name != "f_20261031-120000" || !objs[0].ModTime.IsZero() ||
		!objs[1].ModTime.Equal(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("unexpected list %+v", objs)
	}
	if err := s.Put(ctx, "f_x", strings.NewReader("ignored")); err != nil || len(game.saves) != 4 {
		t.Errorf("Put must be a no-op, got err %v", err)
	}
	rc, err := s.Get(ctx, "f_20261101-120000")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != saveData {
		t.Errorf("Get returned %q", b)
	}
	if err := s.Delete(ctx, "f"); err != nil || !slices.Equal(game.deleted, []string{"f"}) {
		t.Errorf("Delete: err %v, deleted %v", err, game.deleted)
	}
}

func TestApplyRetention(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	objs := []Object{
		{Name: "s1", ModTime: now.Add(-100 * time.Hour)},
		{Name: "s2", ModTime: now.Add(-80 * time.Hour)},
		{Name: "s3"}, // undated
		{Name: "s4", ModTime: now.Add(-time.Hour)},
	}
	maxAge := &metav1.Duration{Duration: 72 * time.Hour}

	tests := []struct {
		name   string
		policy *satisfactoryv1alpha1.SaveGameRetentionPolicy
		want   []string // kept
	}{
		{name: "no policy", want: []string{"s1", "s2", "s3", "s4"}},
		{name: "maxCount", policy: &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](3)},
			want: []string{"s2", "s3", "s4"}},
		{name: "maxAge keeps undated", policy: &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxAge: maxAge},
			want: []string{"s3", "s4"}},
		{name: "both", policy: &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](1), MaxAge: maxAge},
			want: []string{"s4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newMemDest("s1", "s2", "s3", "s4")
			kept, err := applyRetention(context.Background(), d, slices.Clone(objs), tt.policy, now)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, o := range kept {
				names = append(names, o.Name)
			}
			left, _ := d.List(context.Background(), "")
			if !slices.Equal(names, tt.want) || len(left) != len(tt.want) {
				t.Errorf("kept %v (destination holds %d), want %v", names, len(left), tt.want)
			}
		})
	}
}

func TestRunDo(t *testing.T) {
	putBackoff = 0
	const run = "f_20261101-120000"
	const leftover = "f_20261031-120000"
	sum := sha256.Sum256([]byte(saveData))
	ctx := context.Background()

	newRun := func(t *testing.T, game *fakeGame, targets ...Target) Run {
		return Run{Game: game.client(t), SaveName: run, Prefix: "f", Targets: targets, Dir: t.TempDir(), Now: time.Now()}
	}

	t.Run("server and nas succeed", func(t *testing.T) {
		game := &fakeGame{}
		c := game.client(t)
		nas := newMemDest()
		r := Run{Game: c, SaveName: run, Prefix: "f", Dir: t.TempDir(), Now: time.Now(), Targets: []Target{
			{Name: "server", Dest: &Server{Client: c},
				Retention: &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](3)}},
			{Name: "nas", Dest: nas},
		}}
		res := r.Do(ctx)
		if !res.OK() || res.Size != int64(len(saveData)) || res.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("unexpected result %+v", res)
		}
		if game.downloads != 1 || nas.data[run] != saveData || !slices.Equal(game.names(), []string{run}) {
			t.Errorf("downloads %d, nas %v, server %v", game.downloads, nas.data, game.names())
		}
		if res.Destinations[0].Retained != 1 || res.Destinations[1].Retained != 1 {
			t.Errorf("unexpected retained counts %+v", res.Destinations)
		}
	})

	t.Run("one destination fails: nothing deleted from server", func(t *testing.T) {
		game := &fakeGame{saves: []satisfactory.SaveHeader{{SaveName: leftover}}}
		nas, offsite := newMemDest(leftover), newMemDest(leftover)
		offsite.failPut = true
		res := newRun(t, game, Target{Name: "nas", Dest: nas}, Target{Name: "offsite", Dest: offsite}).Do(ctx)
		if res.OK() || !res.Destinations[0].OK || res.Destinations[1].OK || res.Destinations[1].Error != "put: disk full" {
			t.Fatalf("unexpected result %+v", res)
		}
		if offsite.puts != putAttempts || len(game.deleted) != 0 {
			t.Errorf("puts %d, server deletes %v", offsite.puts, game.deleted)
		}
	})

	t.Run("all succeed without server: run's save and full leftovers removed", func(t *testing.T) {
		const partial = "f_20261030-120000"
		game := &fakeGame{saves: []satisfactory.SaveHeader{{SaveName: leftover}, {SaveName: partial}, {SaveName: "g_1"}}}
		nas, offsite := newMemDest(leftover, partial), newMemDest(leftover)
		res := newRun(t, game, Target{Name: "nas", Dest: nas}, Target{Name: "offsite", Dest: offsite}).Do(ctx)
		if !res.OK() {
			t.Fatalf("unexpected result %+v", res)
		}
		slices.Sort(game.deleted)
		if !slices.Equal(game.deleted, []string{leftover, run}) {
			t.Errorf("server deletes %v, want %v", game.deleted, []string{leftover, run})
		}
	})

	t.Run("save call fails", func(t *testing.T) {
		game := &fakeGame{failSave: true}
		nas := newMemDest()
		res := newRun(t, game, Target{Name: "nas", Dest: nas}).Do(ctx)
		if res.Saved || res.OK() || res.Error == "" || nas.puts != 0 || game.downloads != 0 {
			t.Fatalf("unexpected result %+v (puts %d)", res, nas.puts)
		}
		if len(res.Destinations) != 1 || res.Destinations[0].OK || res.Destinations[0].Error == "" {
			t.Errorf("expected one failed destination entry, got %+v", res.Destinations)
		}
	})
}

func TestTruncate(t *testing.T) {
	if got := truncate(strings.Repeat("ä", 200)); len(got) > maxErrorLen || !strings.HasPrefix(got, "ää") {
		t.Errorf("truncate gave %d bytes", len(got))
	}
}
