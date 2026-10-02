// 网络收音机 — C1-Slim 原生网络电台应用。
// 方向键选台，OK 播放/停止，左右键或音量键调节音量，BACK/HOME 退出。
package main

import (
	_ "embed"
	"image"
	"os/exec"

	"c1device"
)

//go:embed assets/pkg-font.bin
var fontData []byte

const (
	screenW  = 296
	screenH  = 152
	rowH     = 16
	listTop  = rowH * 2
	listRows = 6
)

type station struct {
	name string
	url  string
}

var stations = []station{
	{"中国之声", "https://lhttp.qtfm.cn/live/15318317/64k.mp3"},
	{"上海新闻广播", "http://lhttp.qingting.fm/live/270/64k.mp3"},
	{"北京新闻广播", "https://lhttp.qtfm.cn/live/339/64k.mp3"},
	{"广东音乐之声", "https://lhttp.qtfm.cn/live/1260/64k.mp3"},
	{"华语经典500首", "https://lhttp.qtfm.cn/live/5022308/64k.mp3"},
	{"台湾古典音乐", "http://59.120.88.155:8000/live.mp3"},
	{"安徽评书故事", "https://lhttp.qtfm.cn/live/1951/64k.mp3"},
	{"第一财经", "http://lhttp.qingting.fm/live/276/64k.mp3"},
}

type player struct {
	cmd  *exec.Cmd
	playing int // index into stations, -1 = stopped
	note string
}

func render(face *c1device.Face, sel, first, volume int, p *player) c1device.Frame {
	c := c1device.NewCanvas()
	c.Clear()

	// 标题行 + 音量条
	c.DrawText(face, 2, 0, "网络收音机")
	blocks := volume * 10 / 100
	for i := 0; i < 10; i++ {
		r := image.Rect(screenW-14-i*8, 4, screenW-8-i*8, 12)
		if i < blocks {
			c.FillRect(r)
		} else {
			c.DrawRect(r)
		}
	}

	// 列表
	for row := 0; row < listRows; row++ {
		idx := first + row
		if idx >= len(stations) {
			break
		}
		top := listTop + row*rowH
		rect := image.Rect(0, top, screenW, top+rowH)
		name := stations[idx].name
		marker := "  "
		if p.playing == idx {
			marker = "> "
		}
		if idx == sel {
			c.DrawInvertedTextBar(face, rect, " "+marker+name)
		} else {
			c.DrawText(face, 10, top, marker+name)
		}
	}

	// 状态行
	statusY := screenH - rowH
	lineY := statusY - 1
	c.DrawLine(0, lineY, screenW, lineY)
	if p.note != "" {
		c.DrawText(face, 2, statusY, p.note)
	} else if p.playing >= 0 {
		c.DrawText(face, 2, statusY, "正在播放: "+stations[p.playing].name)
	}
	c.DrawTextRight(face, screenW-2, statusY, "BACK退出")
	return c.Frame(128)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

