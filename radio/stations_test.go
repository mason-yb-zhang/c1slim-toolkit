package main

import (
	"testing"

	"c1device"
)

func TestOfficialAndRetainedStations(t *testing.T) {
	official := map[string]string{
		"639": "中国之声", "640": "经济之声", "641": "音乐之声", "642": "经典音乐广播",
		"692": "环球资讯广播", "653": "中国交通广播", "648": "文艺之声", "654": "中国乡村之声",
		"645": "大湾区之声", "643": "台海之声", "644": "神州之声", "646": "香港之声",
		"647": "民族之声", "649": "老年之声", "664": "南海之声", "734": "英语资讯广播 CGTN Radio",
		"650": "藏语广播", "651": "维吾尔语广播", "655": "哈萨克语广播",
	}
	retained := map[string]string{
		"上海新闻广播":   "http://lhttp.qingting.fm/live/270/64k.mp3",
		"北京新闻广播":   "https://lhttp.qtfm.cn/live/339/64k.mp3",
		"广东音乐之声":   "https://lhttp.qtfm.cn/live/1260/64k.mp3",
		"华语经典500首": "https://lhttp.qtfm.cn/live/5022308/64k.mp3",
		"台湾古典音乐":   "http://59.120.88.155:8000/live.mp3",
		"安徽评书故事":   "https://lhttp.qtfm.cn/live/1951/64k.mp3",
		"第一财经":     "http://lhttp.qingting.fm/live/276/64k.mp3",
	}
	if len(stations) != 26 {
		t.Fatalf("station count = %d", len(stations))
	}
	seen := make(map[string]bool)
	for i, s := range stations {
		if seen[s.name] {
			t.Errorf("duplicate station %s", s.name)
		}
		seen[s.name] = true
		if i < 19 {
			if official[s.broadcastID] != s.name || s.url != "" {
				t.Errorf("invalid official station %+v", s)
			}
			delete(official, s.broadcastID)
		} else {
			if retained[s.name] != s.url || s.broadcastID != "" {
				t.Errorf("changed retained station %+v", s)
			}
			delete(retained, s.name)
		}
	}
	if len(official) != 0 || len(retained) != 0 {
		t.Fatalf("missing stations: %v %v", official, retained)
	}
}

func TestEveryStationRendersAcrossListPages(t *testing.T) {
	face, err := c1device.NewBitmapFace(fontData, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	for sel := range stations {
		p := newPlayer(nil)
		p.playing = sel
		p.note = ""
		first := max(0, sel-listRows+1)
		frame := render(face, sel, first, 32, &p)
		if len(frame) != 5624 {
			t.Errorf("invalid frame at station %d", sel)
		}
	}
}
