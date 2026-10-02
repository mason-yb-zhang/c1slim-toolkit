package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type yuntingTestRoundTripper func(*http.Request) (*http.Response, error)

func (f yuntingTestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func yuntingTestClient(t *testing.T, respond yuntingTestRoundTripper) *http.Client {
	t.Helper()
	return &http.Client{Transport: yuntingTestRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "ytmsout.radio.cn" || req.URL.Path != "/web/appBroadcast/list" {
			t.Errorf("unexpected catalog endpoint: %s", req.URL)
			return nil, errors.New("unexpected catalog endpoint")
		}
		return respond(req)
	})}
}

func yuntingTestResponse(req *http.Request, code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func yuntingTestCatalog(id, streamURL string) string {
	return fmt.Sprintf(`{"code":0,"data":[{"contentId":%q,"title":"中国之声","playUrlLow":%q}]}`, id, streamURL)
}

func TestYuntingRequestSignatureAndContext(t *testing.T) {
	now := time.UnixMilli(1700000000123)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := yuntingRequest(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.Host != "ytmsout.radio.cn" || req.URL.Path != "/web/appBroadcast/list" {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
	}
	if req.URL.RawQuery != "categoryId=0&provinceCode=0" {
		t.Errorf("query must be sorted and contain only catalog filters: %q", req.URL.RawQuery)
	}
	fields := url.Values{}
	fields.Set("provinceCode", "0")
	fields.Set("timestamp", "1700000000123")
	fields.Set("categoryId", "0")
	hash := md5.Sum([]byte(fields.Encode() + "&key=f0fc4c668392f9f9a447e48584c214ee"))
	wantSign := strings.ToUpper(hex.EncodeToString(hash[:]))
	for name, want := range map[string]string{
		"timestamp": "1700000000123", "sign": wantSign,
		"Content-Type": "application/json", "equipmentId": "0000", "platformCode": "WEB",
	} {
		if got := req.Header.Get(name); got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
	cancel()
	if !errors.Is(req.Context().Err(), context.Canceled) {
		t.Fatal("request does not carry caller cancellation")
	}
}

func TestYuntingResolveOfficialStringIDAndFreshURL(t *testing.T) {
	calls := 0
	var signs []string
	client := yuntingTestClient(t, func(req *http.Request) (*http.Response, error) {
		calls++
		signs = append(signs, req.Header.Get("sign"))
		wantTimestamp := strconv.FormatInt(1700000000123+int64(calls-1), 10)
		if req.Header.Get("timestamp") != wantTimestamp {
			t.Errorf("timestamp = %q, want %q", req.Header.Get("timestamp"), wantTimestamp)
		}
		streamURL := fmt.Sprintf("http://ytlive.radio.cn/130/radios/10639/index_10639.m3u8?type=1&key=test%d&time=123", calls)
		body := `{"code":0,"message":"success","data":[{"contentId":"640","playUrlLow":"http://ytlive.radio.cn/130/radios/10640/index_10640.m3u8"},{"contentId":"639","title":"中国之声","playUrlLow":` + strconv.Quote(streamURL) + `,"mp3PlayUrlLow":"http://ytcast2.radio.cn/110/radios/30639/index_30639.m3u8"}]}`
		return yuntingTestResponse(req, http.StatusOK, body), nil
	})
	for i := 0; i < 2; i++ {
		got, err := resolveYuntingWithClient(context.Background(), "639", client, time.UnixMilli(1700000000123+int64(i)))
		want := fmt.Sprintf("http://ytlive.radio.cn/130/radios/10639/index_10639.m3u8?type=1&key=test%d&time=123", i+1)
		if err != nil || got != want {
			t.Fatalf("resolve call %d = %q, %v; want %q", i+1, got, err, want)
		}
	}
	if calls != 2 || len(signs) != 2 || signs[0] == signs[1] {
		t.Fatalf("each resolution must fetch and sign anew: calls=%d, signatures=%v", calls, signs)
	}
}

func TestYuntingResolveFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		id     string
		want   string
	}{
		{"http", 503, `{}`, "639", "HTTP 503"},
		{"api", 200, `{"code":123,"data":[]}`, "639", "error 123"},
		{"malformed", 200, `{"code":`, "639", "JSON"},
		{"trailing-json", 200, `{"code":0,"data":[]} {}`, "639", "JSON"},
		{"numeric-id", 200, `{"code":0,"data":[{"contentId":639}]}`, "639", "JSON"},
		{"oversized", 200, strings.Repeat(" ", hlsPlaylistLimit+1), "639", "size limit"},
		{"empty-url", 200, yuntingTestCatalog("639", ""), "639", "URL"},
		{"unknown-id", 200, yuntingTestCatalog("639", "http://ytlive.radio.cn/index.m3u8"), "999999", "not available"},
		{"empty-data", 200, `{"code":0,"data":[]}`, "639", "not available"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := yuntingTestClient(t, func(req *http.Request) (*http.Response, error) {
				return yuntingTestResponse(req, tc.status, tc.body), nil
			})
			got, err := resolveYuntingWithClient(context.Background(), tc.id, client, time.UnixMilli(1700000000123))
			if err == nil || !strings.Contains(err.Error(), tc.want) || got != "" {
				t.Fatalf("got %q, %v; want error containing %q and no URL", got, err, tc.want)
			}
		})
	}
}

func TestYuntingRejectsUnsafeStreamURLs(t *testing.T) {
	for _, raw := range []string{
		"https://example.invalid/live.m3u8", "http://127.0.0.1/live.m3u8",
		"https://evilradio.cn/live.m3u8", "https://radio.cn.example.invalid/live.m3u8",
		"https://user:pass@ytlive.radio.cn/live.m3u8", "file:///tmp/audio",
		"https://ytlive.radio.cn/live.m3u8#fragment", "/relative.m3u8", "https://%zz/live",
	} {
		t.Run(raw, func(t *testing.T) {
			client := yuntingTestClient(t, func(req *http.Request) (*http.Response, error) {
				return yuntingTestResponse(req, 200, yuntingTestCatalog("639", raw)), nil
			})
			got, err := resolveYuntingWithClient(context.Background(), "639", client, time.UnixMilli(1700000000123))
			if err == nil || got != "" {
				t.Fatalf("unsafe URL accepted: %q, %v", got, err)
			}
		})
	}
}

func TestYuntingCatalogAtSizeLimit(t *testing.T) {
	want := "http://ytlive.radio.cn/130/radios/10639/index_10639.m3u8"
	body := yuntingTestCatalog("639", want)
	body += strings.Repeat(" ", hlsPlaylistLimit-len(body))
	client := yuntingTestClient(t, func(req *http.Request) (*http.Response, error) {
		return yuntingTestResponse(req, 200, body), nil
	})
	got, err := resolveYuntingWithClient(context.Background(), "639", client, time.UnixMilli(1700000000123))
	if err != nil || got != want {
		t.Fatalf("exact size limit rejected: %q, %v", got, err)
	}
}

func TestYuntingResolveCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	client := yuntingTestClient(t, func(req *http.Request) (*http.Response, error) {
		close(entered)
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	done := make(chan error, 1)
	go func() {
		_, err := resolveYuntingWithClient(ctx, "639", client, time.UnixMilli(1700000000123))
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request never reached offline transport")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation not preserved: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not stop resolution")
	}
}

func TestYuntingTransportError(t *testing.T) {
	want := errors.New("offline transport failed")
	client := yuntingTestClient(t, func(*http.Request) (*http.Response, error) { return nil, want })
	got, err := resolveYuntingWithClient(context.Background(), "639", client, time.UnixMilli(1700000000123))
	if !errors.Is(err, want) || got != "" {
		t.Fatalf("transport error not preserved: %q, %v", got, err)
	}
}
