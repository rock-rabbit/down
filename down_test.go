package down

import (
	"strings"
	"testing"
)

// func TestDown(t *testing.T) {
// 	// 创建一个基本下载信息
// 	meta := down.NewMeta("http://downloadtest.kdatacenter.com/100MB", "./tmp", "")
// 	// 添加一个请求头
// 	meta.Header.Set("referer", "http://www.68wu.cn/")
// 	// down.Default 为默认配置的下载器, 你可以查看 Down 结构体配置自己的下载器
// 	// Down 和 Mata 结构体可复用, 多线程安全
// 	// 给下载器添加进度条打印的 Hook
// 	down.Default.AddHook(down.DefaultBarHook)
// 	// 执行下载, 你也可以使用 RunContext 传递一个 Context
// 	err := down.Default.Run(meta)
// 	if err != nil {
// 		log.Panic(err)
// 	}
// }

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
			t.Errorf("过滤掉非法字符失败, 输出 %s, 应输出 %s", tmp, v.out)
		}
	}
}
