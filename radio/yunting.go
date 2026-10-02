package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const yuntingEndpoint = "https://ytmsout.radio.cn/web/appBroadcast/list?categoryId=0&provinceCode=0"

func yuntingRequest(ctx context.Context, now time.Time) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, yuntingEndpoint, nil)
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(now.UnixMilli(), 10)
	// This is the public web client's request signature, not a password or a security hash.
	sign := md5.Sum([]byte("categoryId=0&provinceCode=0&timestamp=" + timestamp + "&key=f0fc4c668392f9f9a447e48584c214ee"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("equipmentId", "0000")
	req.Header.Set("platformCode", "WEB")
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("sign", strings.ToUpper(fmt.Sprintf("%x", sign)))
	return req, nil
}

func resolveYuntingWithClient(ctx context.Context, id string, client *http.Client, now time.Time) (string, error) {
	req, err := yuntingRequest(ctx, now)
	if err != nil {
		return "", err
	}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Yunting catalog: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Yunting catalog HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, hlsPlaylistLimit+1))
	if err != nil {
		return "", err
	}
	if len(data) > hlsPlaylistLimit {
		return "", fmt.Errorf("Yunting catalog exceeds size limit")
	}
	var catalog struct {
		Code int `json:"code"`
		Data []struct {
			ID  string `json:"contentId"`
			URL string `json:"playUrlLow"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return "", fmt.Errorf("Yunting catalog JSON: %w", err)
	}
	if catalog.Code != 0 {
		return "", fmt.Errorf("Yunting catalog error %d", catalog.Code)
	}
	for _, channel := range catalog.Data {
		if channel.ID == id {
			u, err := hlsResolveURL(nil, channel.URL)
			if err != nil {
				return "", err
			}
			return u.String(), nil
		}
	}
	return "", fmt.Errorf("Yunting channel %s is not available", id)
}

func streamYunting(ctx context.Context, id, caPath string, out io.Writer) error {
	client, err := newHLSClient(caPath)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	url, err := resolveYuntingWithClient(ctx, id, client, time.Now())
	if err != nil {
		return err
	}
	return streamHLSWithClient(ctx, url, client, out, 0, hlsIdleTimeout)
}
