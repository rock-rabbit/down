package down_test

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

// func TestGetFileName(t *testing.T) {
// 	u, _ := url.Parse("https://bizsec-auth.alicdn.com/a9b5b21ee64d2b47/Qe9k4XSEr4zqvIg7131/dKAC7hFeQdQ2AWPz7hW_223897437873___hd.mp4?auth_key=1660701664-0-0-3baf9eb4e584ff678d5601649e523975&t=212cbb6e16606989647024260edc28&b=video&p=cloudvideo_http_800000012_2")
// 	if u != nil {
// 		us := strings.Split(u.Path, "/")
// 		if len(us) > 0 {
// 			name := us[len(us)-1]
// 			fmt.Println(name)
// 		}

// 	}
// }

// // EnableTestServer 启动测试服务
// func EnableTestServer(t *testing.T) {
// 	http.HandleFunc("/test.file", func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Println(r.Method)
// 		// w.Header().Add("")
// 	})
// 	err := http.ListenAndServe(":28372", nil)
// 	if err != nil {
// 		t.Error(err)
// 	}
// }
