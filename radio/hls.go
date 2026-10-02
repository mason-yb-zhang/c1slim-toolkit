package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	hlsPlaylistLimit = 512 * 1024
	hlsSegmentLimit  = 8 * 1024 * 1024
	hlsWindowLimit   = 64
	hlsIdleTimeout   = 30 * time.Second
)

func streamHLS(ctx context.Context, rawURL, caPath string, out io.Writer) error {
	if _, err := hlsResolveURL(nil, rawURL); err != nil {
		return err
	}
	client, err := newHLSClient(caPath)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	return streamHLSWithClient(ctx, rawURL, client, out, 0, hlsIdleTimeout)
}

func newHLSClient(caPath string) (*http.Client, error) {
	pem, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("HLS CA bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("HLS CA bundle contains no valid certificates")
	}
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: hlsCheckRedirect}, nil
}

func hlsResolveURL(base *url.URL, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid HLS URL: %w", err)
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" {
		return nil, fmt.Errorf("HLS URL must be an HTTP(S) URL without credentials or fragment")
	}
	host := strings.ToLower(u.Hostname())
	allowed := false
	for _, domain := range []string{"cnr.cn", "cri.cn", "100ycdn.com", "radio.cn"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			allowed = true
		}
	}
	if !allowed || net.ParseIP(host) != nil {
		return nil, fmt.Errorf("HLS host %q is not allowed", host)
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return nil, fmt.Errorf("invalid HLS hostname %q", host)
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return nil, fmt.Errorf("invalid HLS hostname %q", host)
			}
		}
	}
	if base != nil && base.Scheme == "https" && u.Scheme != "https" {
		return nil, fmt.Errorf("HLS HTTPS to HTTP downgrade is forbidden")
	}
	return u, nil
}

func hlsCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 3 {
		return fmt.Errorf("HLS redirect limit (3) exceeded")
	}
	var base *url.URL
	if len(via) != 0 {
		base = via[len(via)-1].URL
	}
	_, err := hlsResolveURL(base, req.URL.String())
	return err
}

type hlsSegment struct {
	sequence uint64
	url      *url.URL
}

type hlsPlaylist struct {
	master   *url.URL
	segments []hlsSegment
	target   time.Duration
	ended    bool
}

func parseHLSPlaylist(data []byte, base *url.URL) (hlsPlaylist, error) {
	var p hlsPlaylist
	if len(data) > hlsPlaylistLimit {
		return p, fmt.Errorf("HLS playlist exceeds 512 KiB")
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if strings.TrimSpace(lines[0]) != "#EXTM3U" {
		return p, fmt.Errorf("invalid HLS playlist: missing EXTM3U header")
	}
	var sequence, count uint64
	var haveSequence, pendingSegment, pendingMaster, isMaster bool
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case line == "#EXT-X-KEY" || line == "#EXT-X-SESSION-KEY":
			return p, fmt.Errorf("invalid HLS encryption declaration: missing METHOD")
		case strings.HasPrefix(line, "#EXT-X-KEY:") || strings.HasPrefix(line, "#EXT-X-SESSION-KEY:"):
			attrs := strings.Split(strings.SplitN(line, ":", 2)[1], ",")
			if len(attrs) != 1 || strings.TrimSpace(attrs[0]) != "METHOD=NONE" {
				return p, fmt.Errorf("unsupported HLS encryption: only METHOD=NONE is supported")
			}
		case line == "#EXT-X-MAP" || strings.HasPrefix(line, "#EXT-X-MAP:"):
			return p, fmt.Errorf("unsupported HLS EXT-X-MAP (fragmented MP4)")
		case strings.HasPrefix(line, "#EXT-X-BYTERANGE:") || line == "#EXT-X-GAP" || strings.HasPrefix(line, "#EXT-X-SKIP:") || line == "#EXT-X-I-FRAMES-ONLY":
			return p, fmt.Errorf("unsupported HLS tag: %s", line)
		case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
			if haveSequence || count != 0 {
				return p, fmt.Errorf("invalid HLS media sequence placement")
			}
			var err error
			sequence, err = strconv.ParseUint(strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"), 10, 64)
			if err != nil {
				return p, fmt.Errorf("invalid HLS media sequence: %w", err)
			}
			haveSequence = true
		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
			n, err := strconv.ParseUint(strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"), 10, 32)
			if err != nil || n == 0 {
				return p, fmt.Errorf("invalid HLS target duration")
			}
			p.target = time.Duration(n) * time.Second
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			if pendingMaster || pendingSegment || count != 0 {
				return p, fmt.Errorf("invalid HLS mixed or incomplete master playlist")
			}
			pendingMaster, isMaster = true, true
		case strings.HasPrefix(line, "#EXTINF:"):
			if pendingSegment || isMaster || p.ended {
				return p, fmt.Errorf("invalid HLS segment declaration")
			}
			duration := strings.SplitN(strings.TrimPrefix(line, "#EXTINF:"), ",", 2)[0]
			n, err := strconv.ParseFloat(duration, 64)
			if err != nil || n <= 0 || math.IsInf(n, 0) || math.IsNaN(n) {
				return p, fmt.Errorf("invalid HLS segment duration")
			}
			pendingSegment = true
		case line == "#EXT-X-ENDLIST":
			if pendingSegment || isMaster {
				return p, fmt.Errorf("invalid HLS ENDLIST placement")
			}
			p.ended = true
		case strings.HasPrefix(line, "#"):
			continue
		default:
			u, err := hlsResolveURL(base, line)
			if err != nil {
				return p, err
			}
			if pendingMaster {
				if p.master == nil {
					p.master = u
				}
				pendingMaster = false
			} else if pendingSegment {
				if count > math.MaxUint64-sequence {
					return p, fmt.Errorf("HLS media sequence overflow")
				}
				if len(p.segments) == hlsWindowLimit {
					copy(p.segments, p.segments[1:])
					p.segments = p.segments[:hlsWindowLimit-1]
				}
				p.segments = append(p.segments, hlsSegment{sequence + count, u})
				count++
				pendingSegment = false
			} else {
				return p, fmt.Errorf("invalid HLS URI without EXTINF or STREAM-INF")
			}
		}
	}
	if pendingMaster || pendingSegment {
		return p, fmt.Errorf("incomplete HLS playlist: missing URI")
	}
	if p.master == nil && len(p.segments) == 0 {
		return p, fmt.Errorf("HLS playlist has no segments")
	}
	if p.master == nil && !p.ended && p.target == 0 {
		return p, fmt.Errorf("live HLS playlist missing target duration")
	}
	return p, nil
}

func hlsGet(ctx context.Context, client *http.Client, u *url.URL) (*http.Response, error) {
	if _, err := hlsResolveURL(nil, u.String()); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HLS request %s: %w", u.Redacted(), err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HLS request %s: HTTP %d", u.Redacted(), resp.StatusCode)
	}
	return resp, nil
}

func hlsLoadPlaylist(ctx context.Context, client *http.Client, u *url.URL) (hlsPlaylist, *url.URL, error) {
	resp, err := hlsGet(ctx, client, u)
	if err != nil {
		return hlsPlaylist{}, u, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, hlsPlaylistLimit+1))
	if err != nil {
		return hlsPlaylist{}, u, fmt.Errorf("read HLS playlist: %w", err)
	}
	p, err := parseHLSPlaylist(data, resp.Request.URL)
	return p, resp.Request.URL, err
}

func hlsCopySegment(ctx context.Context, client *http.Client, u *url.URL, out io.Writer) error {
	resp, err := hlsGet(ctx, client, u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength > hlsSegmentLimit {
		return fmt.Errorf("HLS segment exceeds 8 MiB")
	}
	n, err := io.Copy(out, io.LimitReader(resp.Body, hlsSegmentLimit))
	if err != nil {
		return fmt.Errorf("copy HLS segment: %w", err)
	}
	var probe [1]byte
	more, err := io.ReadFull(resp.Body, probe[:])
	if more != 0 {
		return fmt.Errorf("HLS segment exceeds 8 MiB")
	}
	if err != nil && err != io.EOF {
		return fmt.Errorf("read HLS segment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("HLS segment is empty")
	}
	return nil
}

func streamHLSWithClient(ctx context.Context, rawURL string, client *http.Client, out io.Writer, pollOverride, idleTimeout time.Duration) error {
	u, err := hlsResolveURL(nil, rawURL)
	if err != nil {
		return err
	}
	if idleTimeout <= 0 {
		idleTimeout = hlsIdleTimeout
	}
	c := *client
	c.CheckRedirect = hlsCheckRedirect
	if c.Timeout <= 0 || c.Timeout > 20*time.Second {
		c.Timeout = 20 * time.Second
	}
	lastNew := time.Now()
	var lastSequence uint64
	started := false
	masterDepth := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Since(lastNew) >= idleTimeout {
			return fmt.Errorf("HLS no new segments for %s", idleTimeout)
		}
		requestCtx, cancel := context.WithDeadline(ctx, lastNew.Add(idleTimeout))
		p, finalURL, err := hlsLoadPlaylist(requestCtx, &c, u)
		cancel()
		if err != nil {
			if ctx.Err() == nil && time.Since(lastNew) >= idleTimeout {
				return fmt.Errorf("HLS no new segments for %s: %w", idleTimeout, err)
			}
			return fmt.Errorf("HLS playlist: %w", err)
		}
		u = finalURL
		if p.master != nil {
			masterDepth++
			if masterDepth > 3 || started {
				return fmt.Errorf("HLS master nesting exceeds 3 or media changed to master")
			}
			u = p.master
			continue
		}
		segments := p.segments
		if !started && len(segments) > 3 {
			segments = segments[len(segments)-3:]
		}
		for _, segment := range segments {
			if started && segment.sequence <= lastSequence {
				continue
			}
			requestCtx, cancel := context.WithDeadline(ctx, lastNew.Add(idleTimeout))
			err := hlsCopySegment(requestCtx, &c, segment.url, out)
			cancel()
			if err != nil {
				if ctx.Err() == nil && time.Since(lastNew) >= idleTimeout {
					return fmt.Errorf("HLS no new segments for %s: %w", idleTimeout, err)
				}
				return fmt.Errorf("HLS segment %d: %w", segment.sequence, err)
			}
			lastSequence, started, lastNew = segment.sequence, true, time.Now()
		}
		if p.ended {
			return nil
		}
		wait := p.target / 2
		if wait < time.Second {
			wait = time.Second
		}
		if wait > 10*time.Second {
			wait = 10 * time.Second
		}
		if pollOverride > 0 {
			wait = pollOverride
		}
		if remaining := time.Until(lastNew.Add(idleTimeout)); wait > remaining {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
