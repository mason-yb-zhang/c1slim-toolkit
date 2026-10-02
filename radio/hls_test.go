package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func hlsTestClient(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	transport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: time.Second}
}

func hlsTestRun(t *testing.T, client *http.Client, path string, out io.Writer) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return streamHLSWithClient(ctx, "http://live.cnr.cn"+path, client, out, time.Millisecond, 500*time.Millisecond)
}

func hlsTestMedia(sequence, count int, ended bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:%d\n", sequence)
	for i := sequence; i < sequence+count; i++ {
		fmt.Fprintf(&b, "#EXTINF:2,\n%d.ts\n", i)
	}
	if ended {
		b.WriteString("#EXT-X-ENDLIST\n")
	}
	return b.String()
}

func TestHLSRelativeMasterAndLiveSequence(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	polls := 0
	client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/root.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\naudio/live.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=2\nunused.m3u8\n")
		case "/audio/live.m3u8":
			polls++
			switch polls {
			case 1, 2:
				fmt.Fprint(w, hlsTestMedia(10, 5, false))
			default:
				fmt.Fprint(w, hlsTestMedia(12, 4, true))
			}
		default:
			fmt.Fprint(w, strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/audio/"), ".ts"), ",")
		}
	})
	var out bytes.Buffer
	if err := hlsTestRun(t, client, "/root.m3u8", &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12,13,14,15," {
		t.Fatalf("unexpected stream: %q", out.String())
	}
	mu.Lock()
	defer mu.Unlock()
	want := "/root.m3u8 /audio/live.m3u8 /audio/12.ts /audio/13.ts /audio/14.ts /audio/live.m3u8 /audio/live.m3u8 /audio/15.ts"
	if strings.Join(paths, " ") != want {
		t.Fatalf("requests: %v", paths)
	}
}

func TestHLSRedirectRelativeBase(t *testing.T) {
	client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/nested/list", http.StatusFound)
		case "/nested/list":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-KEY:METHOD=NONE\n#EXTINF:1,\n../audio.aac?token=x\n#EXT-X-ENDLIST\n")
		case "/audio.aac":
			if r.URL.RawQuery != "token=x" {
				http.Error(w, "missing query", 400)
				return
			}
			fmt.Fprint(w, "AAC")
		default:
			http.NotFound(w, r)
		}
	})
	var out bytes.Buffer
	if err := hlsTestRun(t, client, "/start", &out); err != nil || out.String() != "AAC" {
		t.Fatalf("output=%q error=%v", out.String(), err)
	}
}

func TestHLSMasterDepth(t *testing.T) {
	for _, depth := range []int{3, 4} {
		t.Run(strconv.Itoa(depth), func(t *testing.T) {
			client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/0.ts" {
					fmt.Fprint(w, "TS")
					return
				}
				n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
				if n < depth {
					fmt.Fprintf(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\n%d\n", n+1)
				} else {
					fmt.Fprint(w, hlsTestMedia(0, 1, true))
				}
			})
			err := hlsTestRun(t, client, "/0", io.Discard)
			if depth == 3 && err != nil || depth == 4 && (err == nil || !strings.Contains(err.Error(), "nesting")) {
				t.Fatalf("depth %d: %v", depth, err)
			}
		})
	}
}

func TestHLSRejectPlaylists(t *testing.T) {
	base, _ := url.Parse("https://live.cnr.cn/a/list.m3u8")
	cases := map[string]string{
		"no segments":      "#EXTM3U\n#EXT-X-TARGETDURATION:2\n",
		"encryption":       "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key\"\n",
		"METHOD=NONE":      "#EXTM3U\n#EXT-X-KEY:URI=\"key\"\n",
		"MAP":              "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n",
		"BYTERANGE":        "#EXTM3U\n#EXT-X-BYTERANGE:100@0\n",
		"GAP":              "#EXTM3U\n#EXT-X-GAP\n",
		"SKIP":             "#EXTM3U\n#EXT-X-SKIP:SKIPPED-SEGMENTS=1\n",
		"header":           "<html>not HLS</html>",
		"missing URI":      "#EXTM3U\n#EXTINF:1,\n",
		"without EXTINF":   "#EXTM3U\na.ts\n",
		"target duration":  "#EXTM3U\n#EXTINF:1,\na.ts\n",
		"media sequence":   "#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:-1\n",
		"overflow":         "#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:18446744073709551615\n#EXTINF:1,\na.ts\n#EXTINF:1,\nb.ts\n#EXT-X-ENDLIST\n",
		"segment duration": "#EXTM3U\n#EXTINF:NaN,\na.ts\n",
		"downgrade":        "#EXTM3U\n#EXTINF:1,\nhttp://cnr.cn/a.ts\n#EXT-X-ENDLIST\n",
		"not allowed":      "#EXTM3U\n#EXTINF:1,\nhttps://evil.test/a.ts\n#EXT-X-ENDLIST\n",
	}
	for want, body := range cases {
		t.Run(want, func(t *testing.T) {
			_, err := parseHLSPlaylist([]byte(body), base)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want %q error, got %v", want, err)
			}
		})
	}
}

func TestHLSHTTPAndSizeErrors(t *testing.T) {
	cases := []struct {
		name, playlist, segment, want string
		playlistStatus, segmentStatus int
		chunked                       bool
	}{
		{name: "playlist HTTP", playlistStatus: 404, want: "HTTP 404"},
		{name: "segment HTTP", playlist: hlsTestMedia(0, 1, true), segmentStatus: 503, want: "HTTP 503"},
		{name: "empty playlist", playlist: "#EXTM3U\n", want: "no segments"},
		{name: "encryption", playlist: "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES\n", want: "encryption"},
		{name: "playlist size", playlist: "#EXTM3U\n#" + strings.Repeat("x", hlsPlaylistLimit), want: "512 KiB"},
		{name: "segment size", playlist: hlsTestMedia(0, 1, true), segment: strings.Repeat("x", hlsSegmentLimit+1), want: "8 MiB"},
		{name: "chunked segment size", playlist: hlsTestMedia(0, 1, true), segment: strings.Repeat("x", hlsSegmentLimit+1), chunked: true, want: "8 MiB"},
		{name: "empty segment", playlist: hlsTestMedia(0, 1, true), want: "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				body, status := tc.playlist, tc.playlistStatus
				if r.URL.Path != "/list" {
					body, status = tc.segment, tc.segmentStatus
				}
				if status == 0 {
					status = 200
				}
				if !tc.chunked {
					w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				}
				w.WriteHeader(status)
				if tc.chunked {
					w.(http.Flusher).Flush()
				}
				io.WriteString(w, body)
			})
			var out bytes.Buffer
			err := hlsTestRun(t, client, "/list", &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
			if out.Len() > hlsSegmentLimit {
				t.Fatalf("wrote %d bytes", out.Len())
			}
		})
	}
}

func TestHLSTruncatedResponse(t *testing.T) {
	for _, mode := range []string{"playlist", "segment"} {
		t.Run(mode, func(t *testing.T) {
			client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if mode == "segment" && r.URL.Path == "/list" {
					fmt.Fprint(w, hlsTestMedia(0, 1, true))
					return
				}
				w.Header().Set("Content-Length", "100")
				fmt.Fprint(w, "short")
			})
			if err := hlsTestRun(t, client, "/list", io.Discard); !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("truncated response error: %v", err)
			}
		})
	}
}

func TestHLSNetworkFailure(t *testing.T) {
	failed := errors.New("test network unavailable")
	client := &http.Client{Transport: &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) { return nil, failed }}}
	if err := hlsTestRun(t, client, "/list", io.Discard); !errors.Is(err, failed) {
		t.Fatalf("network error lost: %v", err)
	}
}

func TestHLSWindowBound(t *testing.T) {
	base, _ := url.Parse("http://cnr.cn/list")
	p, err := parseHLSPlaylist([]byte(hlsTestMedia(100, 300, true)), base)
	if err != nil || len(p.segments) != 64 || p.segments[0].sequence != 336 || p.segments[63].sequence != 399 {
		t.Fatalf("bounded window: %+v, %v", p, err)
	}
}

func TestHLSURLPolicy(t *testing.T) {
	for _, raw := range []string{"http://cnr.cn/a", "https://a.b.cnr.cn/a", "https://CRI.CN/a", "https://radio.100ycdn.com/a", "http://ytlive.radio.cn/a", "https://radio.cn/a", "http://ytcast2.radio.cn/a"} {
		if _, err := hlsResolveURL(nil, raw); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{"file:///a", "ftp://cnr.cn/a", "http://localhost/a", "http://127.0.0.1/a", "http://[::1]/a", "http://cnr.cn.evil.test/a", "http://evilcnr.cn/a", "http://user@cnr.cn/a", "http://.cnr.cn/a", "http://a..cnr.cn/a", "http://-a.cnr.cn/a", "http://cnr.cn./a", "/relative", "https://cnr.cn/a#x"} {
		if _, err := hlsResolveURL(nil, raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	base, _ := url.Parse("https://cnr.cn/a")
	if _, err := hlsResolveURL(base, "http://cri.cn/b"); err == nil {
		t.Fatal("allowed downgrade")
	}
}

func TestHLSRedirectPolicy(t *testing.T) {
	for _, limit := range []int{3, 4} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
				if n < limit {
					http.Redirect(w, r, fmt.Sprintf("/%d", n+1), 302)
					return
				}
				fmt.Fprint(w, "#EXTM3U\n#EXTINF:1,\na.ts\n#EXT-X-ENDLIST\n")
			})
			u, _ := url.Parse("http://cnr.cn/0")
			client.CheckRedirect = hlsCheckRedirect
			_, _, err := hlsLoadPlaylist(context.Background(), client, u)
			if limit == 3 && err != nil || limit == 4 && (err == nil || !strings.Contains(err.Error(), "redirect limit")) {
				t.Fatalf("%v", err)
			}
		})
	}
	base, _ := http.NewRequest("GET", "https://cnr.cn/list", nil)
	for _, raw := range []string{"http://cnr.cn/list", "https://evil.test/list", "http://127.0.0.1/list"} {
		req, _ := http.NewRequest("GET", raw, nil)
		if err := hlsCheckRedirect(req, []*http.Request{base}); err == nil {
			t.Errorf("allowed %s", raw)
		}
	}
	client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "http://evil.test/list", 302) })
	if err := hlsTestRun(t, client, "/list", io.Discard); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("redirect escaped allowlist: %v", err)
	}
}

func TestHLSCancellation(t *testing.T) {
	for _, mode := range []string{"request", "segment", "body", "refresh"} {
		t.Run(mode, func(t *testing.T) {
			reached := make(chan struct{}, 1)
			client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if mode == "request" || (mode == "segment" || mode == "body") && r.URL.Path != "/list" {
					if mode == "body" {
						fmt.Fprint(w, "partial segment")
						w.(http.Flusher).Flush()
					}
					reached <- struct{}{}
					<-r.Context().Done()
					return
				}
				if r.URL.Path == "/list" {
					fmt.Fprint(w, hlsTestMedia(0, 1, false))
					return
				}
				fmt.Fprint(w, "TS")
				reached <- struct{}{}
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- streamHLSWithClient(ctx, "http://cnr.cn/list", client, io.Discard, time.Second, hlsIdleTimeout)
			}()
			select {
			case <-reached:
			case <-time.After(time.Second):
				t.Fatal("request not reached")
			}
			if mode == "refresh" {
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("got %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation blocked")
			}
		})
	}
}

func TestHLSStalledLive(t *testing.T) {
	client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list" {
			fmt.Fprint(w, hlsTestMedia(0, 1, false))
			return
		}
		fmt.Fprint(w, "TS")
	})
	var out bytes.Buffer
	start := time.Now()
	err := streamHLSWithClient(context.Background(), "http://cnr.cn/list", client, &out, time.Millisecond, 40*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "no new segments") || out.String() != "TS" || time.Since(start) > time.Second {
		t.Fatalf("stalled stream: %q %v", out.String(), err)
	}
}

func TestHLSLargeRefreshWindow(t *testing.T) {
	polls := 0
	client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list" {
			polls++
			if polls == 1 {
				fmt.Fprint(w, hlsTestMedia(0, 1, false))
			} else {
				fmt.Fprint(w, hlsTestMedia(1, 100, true))
			}
			return
		}
		fmt.Fprint(w, "x")
	})
	var out bytes.Buffer
	if err := hlsTestRun(t, client, "/list", &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 65 {
		t.Fatalf("expected initial segment plus 64, got %d", out.Len())
	}
}

type hlsFailWriter struct{}

func (hlsFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestHLSOutputFailure(t *testing.T) {
	client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list" {
			fmt.Fprint(w, hlsTestMedia(0, 1, true))
			return
		}
		fmt.Fprint(w, "TS")
	})
	if err := hlsTestRun(t, client, "/list", hlsFailWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output error lost: %v", err)
	}
}

func TestHLSExactSegmentLimit(t *testing.T) {
	client := hlsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list" {
			fmt.Fprint(w, hlsTestMedia(0, 1, true))
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(hlsSegmentLimit))
		io.Copy(w, strings.NewReader(strings.Repeat("x", hlsSegmentLimit)))
	})
	if err := hlsTestRun(t, client, "/list", io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestHLSInvalidCA(t *testing.T) {
	if err := streamHLS(context.Background(), "https://cnr.cn/list", filepath.Join(t.TempDir(), "missing.pem"), io.Discard); err == nil || !strings.Contains(err.Error(), "CA bundle") {
		t.Fatalf("missing CA: %v", err)
	}
	path := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := streamHLS(context.Background(), "https://cnr.cn/list", path, io.Discard); err == nil || !strings.Contains(err.Error(), "no valid certificates") {
		t.Fatalf("invalid CA: %v", err)
	}
}

func TestHLSTLSVerifiesHostnameAndChain(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "HLS test"}, DNSNames: []string{"cnr.cn"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list" {
			fmt.Fprint(w, hlsTestMedia(0, 1, true))
			return
		}
		fmt.Fprint(w, "TS")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	defer srv.Close()
	for _, tc := range []struct {
		host  string
		trust bool
		want  string
	}{{"cnr.cn", true, ""}, {"cri.cn", true, "certificate"}, {"cnr.cn", false, "unknown authority"}} {
		t.Run(tc.host+strconv.FormatBool(tc.trust), func(t *testing.T) {
			client, err := newHLSClient(path)
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			transport := client.Transport.(*http.Transport)
			transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
			}
			if !tc.trust {
				transport.TLSClientConfig.RootCAs = x509.NewCertPool()
			}
			err = streamHLSWithClient(context.Background(), "https://"+tc.host+"/list", client, io.Discard, time.Millisecond, time.Second)
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("TLS result: %v", err)
			}
		})
	}
}
