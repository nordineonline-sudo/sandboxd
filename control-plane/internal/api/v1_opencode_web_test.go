package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tastyeffectco/sandboxd/control-plane/internal/store"
)

const (
	owTestKey  = "0123456789abcdef0123456789abcdef" // 32 bytes (already base64-safe chars)
	owTestID   = "01M0FAFHAEDRYWQEP86E1VV7MD"
	owTestIP   = "172.19.0.9"
	owPassword = "f3dd7e5a40308f1c8f3d58e3c220784b" // HMAC-SHA256("0123…def", owTestID) hex[:32]
)

func owTestServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{Store: memStore(t)}
	s.OpencodeWebKey = []byte(owTestKey)
	s.PreviewDomain = "localhost"
	if err := s.Store.Create(context.Background(), &store.Sandbox{
		ID: owTestID, Status: "running",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SetContainerIP(context.Background(), owTestID, owTestIP); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestParseOpencodeIDFromHost(t *testing.T) {
	cases := []struct{ host, want string }{
		{"opencode-01M0FAFHAEDRYWQEP86E1VV7MD.preview.localhost", owTestID},
		{"opencode-01m0fafhaedrywqep86e1vv7md.preview.localhost", owTestID}, // browsers lowercase the host
		{"opencode-01M0FAFHAEDRYWQEP86E1VV7MD.preview.localhost:18080", owTestID},
		{"opencode-01M0FAFHAEDRYWQEP86E1VV7MD.preview.example.com", ""}, // domain arg is "localhost" — must NOT match example.com
		{"opencode-01M0FAFHAEDRYWQEP86E1VV7MD.preview.localhost:443", owTestID},
		{"", ""},
		{"opencode-.preview.localhost", ""},
		{"opencode-notaulid.preview.localhost", ""},
		{"opencode-01M0FAFHAEDRYWQEP86E1VV7MD.localhost", ""},    // no .preview.
		{"opencode-01M0FAFHAEDRYWQEP86E1VV7MD.evil.localhost", ""},
		{"s-01M0FAFHAEDRYWQEP86E1VV7MD-3000.preview.localhost", ""}, // preview shape, not opencode
		{"OPEncode-01M0FAFHAEDRYWQEP86E1VV7MD.preview.localhost", ""},
	}
	for _, c := range cases {
		got := parseOpencodeIDFromHost(c.host, "localhost")
		if got != c.want {
			t.Errorf("parseOpencodeIDFromHost(%q) = %q, want %q", c.host, got, c.want)
		}
	}
}

func TestOpencodeWebAuthorized(t *testing.T) {
	s := owTestServer(t)
	goodTok := opencodeWebAuthToken(s.OpencodeWebKey, owTestID)
	badTok := opencodeWebAuthToken([]byte("different-key-different-key-diff"), owTestID)

	t.Run("auth_token query", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/?auth_token="+goodTok, nil)
		if !opencodeWebAuthorized(r, owPassword) {
			t.Fatal("valid auth_token rejected")
		}
		r = httptest.NewRequest(http.MethodGet, "/?auth_token="+badTok, nil)
		if opencodeWebAuthorized(r, owPassword) {
			t.Fatal("invalid auth_token accepted")
		}
		r = httptest.NewRequest(http.MethodGet, "/", nil)
		if opencodeWebAuthorized(r, owPassword) {
			t.Fatal("no credential accepted")
		}
	})

	t.Run("basic header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		r.Header.Set("Authorization", opencodeWebBasicAuth(s.OpencodeWebKey, owTestID))
		if !opencodeWebAuthorized(r, owPassword) {
			t.Fatal("valid basic rejected")
		}
		r = httptest.NewRequest(http.MethodGet, "/api/health", nil)
		r.Header.Set("Authorization", "Basic "+badTok)
		if opencodeWebAuthorized(r, owPassword) {
			t.Fatal("invalid basic accepted")
		}
	})
}

func TestOpencodeWebURL(t *testing.T) {
	s := owTestServer(t)
	wantHost := "opencode-" + owTestID + ".preview.localhost"
	u := s.opencodeWebURL(owTestID)
	if !strings.HasPrefix(u, "http://"+wantHost+"/?auth_token=") {
		t.Fatalf("url %q missing host prefix", u)
	}
	tok := strings.TrimPrefix(u, "http://"+wantHost+"/?auth_token=")
	if user, pass, ok := decodeAuthToken(tok); !ok || user != opencodeWebUser || pass != owPassword {
		t.Fatalf("url token decodes wrong: %q %q %v", user, pass, ok)
	}

	s.PreviewTLS = true
	s.PublicHTTPPort = "443"
	if !strings.HasPrefix(s.opencodeWebURL(owTestID), "https://"+wantHost+"/?auth_token=") {
		t.Fatal("TLS url should be https without port suffix")
	}
	s.PublicHTTPPort = "18080"
	if !strings.HasPrefix(s.opencodeWebURL(owTestID), "https://"+wantHost+":18080/?auth_token=") {
		t.Fatal("non-default public port should appear in url")
	}
}

func TestOpencodeWebHostHandlerAuthAndState(t *testing.T) {
	t.Run("disabled feature -> 503", func(t *testing.T) {
		s := &Server{Store: memStore(t)}
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = "opencode-" + owTestID + ".preview.localhost"
		w := httptest.NewRecorder()
		s.ServeOpencodeWebHost(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("got %d", w.Code)
		}
	})

	t.Run("unknown host -> 404", func(t *testing.T) {
		s := owTestServer(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = "notanopencodehost.preview.localhost"
		w := httptest.NewRecorder()
		s.ServeOpencodeWebHost(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d", w.Code)
		}
	})

	t.Run("stopped sandbox -> 409", func(t *testing.T) {
		s := owTestServer(t)
		if err := s.Store.MarkStopped(context.Background(), owTestID); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = "opencode-" + owTestID + ".preview.localhost"
		w := httptest.NewRecorder()
		s.ServeOpencodeWebHost(w, r)
		if w.Code != http.StatusConflict {
			t.Fatalf("got %d", w.Code)
		}
	})

	t.Run("missing token -> 401", func(t *testing.T) {
		s := owTestServer(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = "opencode-" + owTestID + ".preview.localhost"
		w := httptest.NewRecorder()
		s.ServeOpencodeWebHost(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("got %d", w.Code)
		}
	})

	t.Run("valid token reaches proxy path (bad gateway upstream)", func(t *testing.T) {
		s := owTestServer(t)
		r := httptest.NewRequest(http.MethodGet, "/?auth_token="+opencodeWebAuthToken(s.OpencodeWebKey, owTestID), nil)
		r.Host = "opencode-" + owTestID + ".preview.localhost"
		w := httptest.NewRecorder()
		s.ServeOpencodeWebHost(w, r)
		// No server on 172.19.0.9:4097 → the ReverseProxy's dial fails and the
		// ErrorHandler emits 502. A 502 proves auth+state passed and it proxied.
		if w.Code != http.StatusBadGateway {
			t.Fatalf("got %d (want 502 from unreachable upstream)", w.Code)
		}
	})

	t.Run("static asset skips client auth (bad gateway upstream)", func(t *testing.T) {
		s := owTestServer(t)
		r := httptest.NewRequest(http.MethodGet, "/assets/index-CQtwhDOb.js", nil)
		r.Host = "opencode-" + owTestID + ".preview.localhost"
		w := httptest.NewRecorder()
		s.ServeOpencodeWebHost(w, r)
		// No auth supplied, but /assets/* is a public static path → proxied
		// anyway; the unreachable test upstream makes it a 502 (not a 401).
		if w.Code != http.StatusBadGateway {
			t.Fatalf("got %d (want 502: static path bypassed client auth)", w.Code)
		}
	})
}

func TestOpencodeWebStaticPath(t *testing.T) {
	static := []string{
		"/assets/index-CQtwhDOb.js",
		"/assets/index-CMLUT3g5.css",
		"/assets/chunk-abc.js",
		"/favicon.ico",
		"/favicon-v3.ico",
		"/favicon-v3.svg",
		"/favicon-96x96-v3.png",
		"/apple-touch-icon-v3.png",
		"/site.webmanifest",
		"/social-share.png",
	}
	auth := []string{"/", "/api/health", "/global/health", "/event", "/message", "/session", "/pty/1/2", "/config", "/files/1/2"}
	for _, p := range static {
		if !opencodeWebStaticPath(p) {
			t.Errorf("opencodeWebStaticPath(%q) = false, want true", p)
		}
	}
	for _, p := range auth {
		if opencodeWebStaticPath(p) {
			t.Errorf("opencodeWebStaticPath(%q) = true, want false", p)
		}
	}
}