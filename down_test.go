package down_test

import (
	"log"
	"testing"
	"time"

	"github.com/rock-rabbit/down"
)

func TestDown(t *testing.T) {
	// 创建一个基本下载信息
	meta := down.NewMeta("https://dldir1.qq.com/qqfile/qq/PCQQ9.6.6/QQ9.6.6.28788.exe", "./tmp", "")
	// 添加一个请求头
	meta.Header.Set("referer", "https://im.qq.com/")
	// down.Default 为默认配置的下载器, 你可以查看 Down 结构体配置自己的下载器
	down.Default.AddHook(down.DefaultBarHook)
	down.Default.ThreadSize = 1024 << 10
	down.Default.ThreadCount = 1
	down.Default.Timeout = time.Second * 50
	// 执行下载, 你也可以使用 RunContext 传递一个 Context
	err := down.Default.Run(meta)
	if err != nil {
		log.Panic(err)
	}
}
