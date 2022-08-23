package down

import (
	"log"
	"testing"
)

func TestDown(t *testing.T) {
	// 创建一个基本下载信息
	meta := NewMeta("https://dldir1.qq.com/qqfile/qq/PCQQ9.6.5/QQ9.6.5.28778.exe", "./tmp", "")
	// 添加一个请求头
	meta.Header.Set("referer", "https://im.qq.com/")
	// down.Default 为默认配置的下载器, 你可以查看 Down 结构体配置自己的下载器
	Default.AddHook(DefaultBarHook)
	Default.ThreadCount = 5
	// 执行下载, 你也可以使用 RunContext 传递一个 Context
	err := Default.Run(meta)
	if err != nil {
		log.Panic(err)
	}
}
