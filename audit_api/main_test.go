// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPingRepondPONGV2(t *testing.T) {
	srv := httptest.NewServer(newRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("attendu 200, obtenu %d", resp.StatusCode)
	}
	if string(body) != "PONG-V2" {
		t.Fatalf("attendu \"PONG-V2\", obtenu %q", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type inattendu: %q", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "7" {
		t.Fatalf("Content-Length attendu 7 (PONG-V2), obtenu %q", cl)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control attendu no-store, obtenu %q", cc)
	}
}

func TestHeadSurPing(t *testing.T) {
	srv := httptest.NewServer(newRouter())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodHead, srv.URL+"/ping", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HEAD /ping: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Le pattern "GET /ping" couvre HEAD : 200, sans corps, mais Content-Length annoncé.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("attendu 200 pour HEAD, obtenu %d", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Fatalf("HEAD ne doit pas renvoyer de corps, obtenu %q", body)
	}
}

func TestMethodesRefusees(t *testing.T) {
	srv := httptest.NewServer(newRouter())
	defer srv.Close()

	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req, _ := http.NewRequest(m, srv.URL+"/ping", strings.NewReader("x"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s /ping: %v", m, err)
		}
		allow := resp.Header.Get("Allow")
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s /ping: attendu 405, obtenu %d", m, resp.StatusCode)
		}
		if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") {
			t.Fatalf("%s /ping: en-tête Allow inattendu %q", m, allow)
		}
	}
}

func TestRoutesInconnues(t *testing.T) {
	srv := httptest.NewServer(newRouter())
	defer srv.Close()

	for _, p := range []string{"/", "/pong", "/ping/", "/ping/PONG", "/v1/events"} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s: attendu 404, obtenu %d", p, resp.StatusCode)
		}
	}
}
