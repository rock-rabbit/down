package down

import (
	"log"
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
