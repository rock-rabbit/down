package down_test

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rock-rabbit/down"
)

func TestDown(t *testing.T) {
	// 创建一个基本下载信息
	meta := down.NewMeta("http://downloadtest.kdatacenter.com/100MB", "./", "")
	// 给下载器添加进度条打印的 Hook
	down.Default.AddHook(&down.BarHook{})
	// 执行下载
	err := down.Default.Run(meta)
	if err != nil {
		log.Panic(err)
	}
}

func TestGetFileName(t *testing.T) {
	u, _ := url.Parse("https://bizsec-auth.alicdn.com/a9b5b21ee64d2b47/Qe9k4XSEr4zqvIg7131/dKAC7hFeQdQ2AWPz7hW_223897437873___hd.mp4?auth_key=1660701664-0-0-3baf9eb4e584ff678d5601649e523975&t=212cbb6e16606989647024260edc28&b=video&p=cloudvideo_http_800000012_2")
	if u != nil {
		us := strings.Split(u.Path, "/")
		if len(us) > 0 {
			name := us[len(us)-1]
			fmt.Println(name)
		}

	}
}

// EnableTestServer 启动测试服务
func EnableTestServer(t *testing.T) {
	http.HandleFunc("/test.file", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Method)
		// w.Header().Add("")
	})
	err := http.ListenAndServe(":28372", nil)
	if err != nil {
		t.Error(err)
	}
}
