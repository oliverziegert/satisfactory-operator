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

// Command fakegame is a stand-in for the Satisfactory dedicated server in e2e tests. It serves
// the save-related HTTPS API functions on :7777 with a self-signed certificate. It reports
// itself as claimed and accepts any admin password. Saves live in memory.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"slices"
	"sync"
	"time"
)

const token = "fake-token"

type saveHeader struct {
	SaveName     string `json:"saveName"`
	SaveDateTime string `json:"saveDateTime"`
}

type server struct {
	mu    sync.Mutex
	saves []saveHeader
}

func main() {
	s := &server{}
	srv := &http.Server{
		Addr:              ":7777",
		Handler:           http.HandlerFunc(s.handle),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{selfSigned()}},
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Print("fake game API listening on :7777")
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Data struct {
			SaveName string `json:"saveName"`
		} `json:"data"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	fn, name := r.URL.Query().Get("function"), req.Data.SaveName
	log.Printf("%s %s", fn, name)

	switch fn {
	case "PasswordlessLogin":
		apiError(w, http.StatusForbidden, "passwordless_login_not_possible")
		return
	case "PasswordLogin":
		writeData(w, map[string]string{"authenticationToken": token})
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+token {
		apiError(w, http.StatusUnauthorized, "invalid_token")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch fn {
	case "QueryServerState":
		writeData(w, map[string]any{"serverGameState": map[string]any{"activeSessionName": "world"}})
	case "SaveGame":
		s.saves = append(s.saves, saveHeader{SaveName: name, SaveDateTime: time.Now().UTC().Format("2006.01.02-15.04.05")})
		w.WriteHeader(http.StatusNoContent)
	case "EnumerateSessions":
		writeData(w, map[string]any{"sessions": []any{map[string]any{"sessionName": "world", "saveHeaders": s.saves}}})
	case "DeleteSaveFile":
		s.saves = slices.DeleteFunc(s.saves, func(h saveHeader) bool { return h.SaveName == name })
		w.WriteHeader(http.StatusNoContent)
	case "DownloadSaveGame":
		if !slices.ContainsFunc(s.saves, func(h saveHeader) bool { return h.SaveName == name }) {
			apiError(w, http.StatusNotFound, "file_not_found")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("fake save " + name))
	default:
		apiError(w, http.StatusBadRequest, "unknown_function")
	}
}

func writeData(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func apiError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"errorCode": code})
}

func selfSigned() tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now(), NotAfter: time.Now().Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
