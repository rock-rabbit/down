package down

import (
	"log"
	"strings"
	"testing"
)

func TestDown(t *testing.T) {
	// https://dldir1.qq.com/qqfile/qq/PCQQ9.6.5/QQ9.6.5.28778.exe
	// http://downloadtest.kdatacenter.com/100MB
	// https://images.unsplash.com/photo-1661127402205-9480179922b3
	// 创建一个基本下载信息
	meta := NewMeta("https://images.unsplash.com/photo-1661127402205-9480179922b3", "./tmp", "")
	// 添加一个请求头
	meta.Header.Set("referer", "https://unsplash.com/photos/LXWH5mdlNhE")
	// down.Default 为默认配置的下载器, 你可以查看 Down 结构体配置自己的下载器
	// Down 和 Mata 结构体可复用, 多线程安全
	// 给下载器添加进度条打印的 Hook
	Default.AddHook(DefaultBarHook)
	Default.ThreadCount = 5
	// 执行下载, 你也可以使用 RunContext 传递一个 Context
	err := Default.Run(meta)
	if err != nil {
		log.Panic(err)
	}
}

func TestThreadTaskSplit(t *testing.T) {
	testData := []struct {
		size, threadSize int64
		out              [][2]int64
	}{
		{0, 0, [][2]int64{}},
		{2048, 1024, [][2]int64{{0, 1024}, {1025, 2048}}},
		{2049, 1024, [][2]int64{{0, 1024}, {1025, 2048}, {2049, 2049}}},
		{2047, 1024, [][2]int64{{0, 1024}, {1025, 2047}}},
		{0, 1024, [][2]int64{}},
	}

	for _, v := range testData {
		tmp := threadTaskSplit(v.size, v.threadSize)
		if len(tmp) != len(v.out) {
			t.Fatalf("size:%d threadSize:%d 任务分割失败, 长度不一致，输出 %v, 应输出 %v", v.size, v.threadSize, tmp, v.out)
		}
		for idx, vv := range tmp {
			if vv != v.out[idx] {
				t.Fatalf("size:%d threadSize:%d 任务分割失败, 输出 %v, 应输出 %v", v.size, v.threadSize, tmp, v.out)
			}
		}
	}
}

func TestGetFileName(t *testing.T) {
	testData := []struct {
		uri                string
		contentDisposition string
		contentType        string
		headinfo           []byte
		out                string
	}{
		{"", "", "", []byte{}, "file"},
		{"test.com", "", "", []byte{}, "file"},
		{"test.com", "attachment;filename=2022-12.xlsx", "", []byte{}, "2022-12.xlsx"},
		{"test.com/file", "", "application/postscript", []byte{}, "file.ai"},
		{"test.com/file", "", "", []byte{80, 75, 3, 4, 20, 0, 0, 0, 8, 0}, "file.zip"},
		{"test.com/2022-12.xlsx?s=521", "", "", []byte{}, "2022-12.xlsx"},
	}
	for _, v := range testData {
		tmp := getFileName(v.uri, v.contentDisposition, v.contentType, v.headinfo)
		if tmp != v.out && !strings.HasPrefix(tmp, v.out) {
			t.Errorf("获取文件名称失败, 输出 %s, 应输出 %s", tmp, v.out)
		}
	}
}
