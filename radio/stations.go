package main

type station struct {
	name        string
	url         string
	broadcastID string
}

var stations = []station{
	{name: "中国之声", broadcastID: "639"},
	{name: "经济之声", broadcastID: "640"},
	{name: "音乐之声", broadcastID: "641"},
	{name: "经典音乐广播", broadcastID: "642"},
	{name: "环球资讯广播", broadcastID: "692"},
	{name: "中国交通广播", broadcastID: "653"},
	{name: "文艺之声", broadcastID: "648"},
	{name: "中国乡村之声", broadcastID: "654"},
	{name: "大湾区之声", broadcastID: "645"},
	{name: "台海之声", broadcastID: "643"},
	{name: "神州之声", broadcastID: "644"},
	{name: "香港之声", broadcastID: "646"},
	{name: "民族之声", broadcastID: "647"},
	{name: "老年之声", broadcastID: "649"},
	{name: "南海之声", broadcastID: "664"},
	{name: "英语资讯广播 CGTN Radio", broadcastID: "734"},
	{name: "藏语广播", broadcastID: "650"},
	{name: "维吾尔语广播", broadcastID: "651"},
	{name: "哈萨克语广播", broadcastID: "655"},
	{name: "上海新闻广播", url: "http://lhttp.qingting.fm/live/270/64k.mp3"},
	{name: "北京新闻广播", url: "https://lhttp.qtfm.cn/live/339/64k.mp3"},
	{name: "广东音乐之声", url: "https://lhttp.qtfm.cn/live/1260/64k.mp3"},
	{name: "华语经典500首", url: "https://lhttp.qtfm.cn/live/5022308/64k.mp3"},
	{name: "台湾古典音乐", url: "http://59.120.88.155:8000/live.mp3"},
	{name: "安徽评书故事", url: "https://lhttp.qtfm.cn/live/1951/64k.mp3"},
	{name: "第一财经", url: "http://lhttp.qingting.fm/live/276/64k.mp3"},
}
