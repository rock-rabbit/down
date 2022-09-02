
## 🎤 简介

零依赖，高性能，可扩展，结构清晰的 HTTP 文件下载器 Golang 包

## 🎉 功能
- 多线程下载
- 单线程下载
- 覆盖下载
- 限速下载
- 多文件同时下载
- 磁盘缓冲区
- 断点续传
- HOOK
- 命令行进度条 HOOK

## 📝 进行中
- 写文档

## 🏍️ 计划
- 文件自动重命名
- 生命周期 HOOK

## 🎊 安装
```bash
# 安装
go get github.com/rock-rabbit/down
# 下载到本地使用，零依赖让这种方式变得极为方便
# ...
```
    
## 🪞 演示

![演示](https://www.68wu.cn/down/demonstration2.gif)
## 🛠 使用方法

最简单的使用方法, 默认会下载到运行目录
``` golang
package main

import "github.com/rock-rabbit/down"

func main(){
	// 执行下载，下载完成后返回 文件存储路径 和 错误信息
	path, err := down.Run("http://downloadtest.kdatacenter.com/100MB")

	// 基于 context 运行下载
	// down.RunContext(ctx, "http://downloadtest.kdatacenter.com/100MB")

	if err != nil {
		log.Panic(err)
	}
	fmt.Println("文件下载完成：", path)
}

// 默认输出目录为 ./，运行后输出:
// 文件下载完成：/Users/rockrabbit/projects/down/tmp/100MB
```


使用命令行进度条 Hook
``` golang
// 设置进度条 Hook ， 这里只是展示一下如何设置，使用默认参数可以不用设置
// 使用人类友好单位，默认是 true
down.DefaultBarHook.FriendlyFormat = true

// 完成后是否隐藏进度条，默认是 false
down.DefaultBarHook.FinishHide = false

// 是否隐藏进度条，默认是 false
down.DefaultBarHook.Hide = false

// 进度条的输出，默认是 os.Stdout
down.DefaultBarHook.Stdout = os.Stdout

// 进度条模板，默认是 {{.CompletedLength}} / {{.TotalLength}} {{.Saucer}} {{.Progress}}% {{.DownloadSpeed}}/s {{.EstimatedTime}} CN:{{.Connections}}
down.DefaultBarHook.Template.Template = "{{.CompletedLength}} / {{.TotalLength}} {{.Saucer}} {{.Progress}}% {{.DownloadSpeed}}/s {{.EstimatedTime}} CN:{{.Connections}}"

// 还可以设置进度条长度
down.DefaultBarHook.Template.BarWidth = 100

// ... 还有很多可以设置，可以查看 BarTemplate 结构体

// 给默认下载器添加进度条 Hook，这是一个全局操作
down.AddHook(down.DefaultBarHook)

// 进度条默认样式:
// 100.00 MB / 100.00 MB [============================>---] 95% 12.06 MB/s 0s CN:1
```


多文件同时下载
``` golang
// 给默认下载器添加进度条 Hook，这是一个全局操作
metaMerging := [][2]string{
	{"http://downloadtest.kdatacenter.com/100MB", "down1.bin"},
	{"http://downloadtest.kdatacenter.com/100MB", "down2.bin"},
}
path, err := down.RunMerging(metaMerging, "./")

// 基于 context 下载
// down.RunMergingContext(ctx, metaMerging, "./")

if err != nil {
	log.Panic(err)
}
fmt.Println("文件下载完成：", path)

// 运行后输出:
// 文件下载完成：[/Users/rockrabbit/projects/down/tmp/down1.bin /Users/rockrabbit/projects/down/tmp/down2.bin]
```

自定义 Meta 下载
``` golang
package main

import "github.com/rock-rabbit/down"

func main(){
	meta := down.NewMeta("http://downloadtest.kdatacenter.com/100MB", "./", "100MB.bin")

	// 自定义 Header
	meta.Header.Set("cookie", "111=111")

	// 请求方式
	meta.Method = http.MethodGet

	// 请求时的 Body
	meta.Body = nil

	// 新建文件的权限
	meta.Perm = 0600

	// 执行下载，下载完成后返回 文件存储路径 和 错误信息
	path, err := down.RunMeta(meta)

	// 基于 context 运行下载
	// down.RunMetaContext(ctx, meta)

	// 多文件同时下载
	// down.RunMergingMeta([]*Meta{meta})

	// 基于 context 多文件同时下载
	// down.RunMergingMetaContext(ctx, []*Meta{meta})

	if err != nil {
		log.Panic(err)
	}
	fmt.Println("文件下载完成：", path)
}

// 运行后输出:
// 文件下载完成：/Users/rockrabbit/projects/down/tmp/100MB.bin
```

自定义下载器
``` golang
mydown := down.New()

mydown.AddHook(down.DefaultBarHook)

path, err := mydown.Run("http://downloadtest.kdatacenter.com/100MB")
if err != nil {
	og.Panic(err)
}

fmt.Println("文件下载完成：", path)

// 运行后输出:
// 文件下载完成：/Users/rockrabbit/projects/down/tmp/100MB
```


下载器的设置
``` golang
// SetSpeedLimit 设置限速，每秒下载字节，默认为 0 不限速
down.SetSpeedLimit(n int)

// SetSendTime 设置给 Hook 发送下载进度的间隔时间，默认为 500ms
down.SetSendTime(n time.Duration)

// SetThreadCount 设置多线程时的最大线程数，默认为 1
down.SetThreadCount(n int)

// SetThreadCount 设置多线程时每个线程下载的最大长度，默认为 20M
down.SetThreadSize(n int)

// SetDiskCache 设置磁盘缓冲区大小，默认为 16M
down.SetDiskCache(n int)

// SetDiskCache 设置当需要创建目录时，是否创建目录，默认为 true
down.SetCreateDir(n bool)

// SetAllowOverwrite 设置是否允许覆盖文件，默认为 true
down.SetAllowOverwrite(n bool)

// SetContinue 设置是否启用断点续传，默认为 true
down.SetContinue(n bool)

// SetAutoSaveTnterval 设置自动保存控制文件的时间，默认为 1 秒
down.SetAutoSaveTnterval(n time.Duration)

// SetConnectTimeout 设置 HTTP 连接请求的超时时间，默认为 5 秒
down.SetConnectTimeout(n time.Duration)

// SetTimeout 设置下载总超时时间，默认为 10 分钟
down.SetTimeout(n time.Duration)

// SetRetryNumber 设置下载最多重试次数，默认为 5
down.SetRetryNumber(n int)

// SetRetryTime 重试时的间隔时间，默认为 0
down.SetRetryTime(n time.Duration)

// SetProxy 设置 Http 代理，默认为 http.ProxyFromEnvironment
down.SetProxy(n func(*http.Request) (*url.URL, error))

// SetTempFileExt 设置临时文件后缀, 默认为 down
down.SetTempFileExt(n string)

// AddHook 添加 Hook 的创建接口
down.AddHook(perhook PerHook)
```


## 🔗 目录结构
```
.
├── LICENSE                   开源协议 MIT
├── Makefile                  快捷命令
├── README.md                 说明文件
├── bar_hook.go               控制台进度条 Hook
├── down.go                   下载器配置
├── export.go                 面向外部的快捷方法
├── go.mod
├── hook.go                   Hook 接口
├── meta.go                   基本下载信息
├── mime.go
├── operation.go              具体的下载实现
├── operation_controlfile.go  控制文件
├── operation_down.go         具体的下载实现
├── operation_file.go         操作文件
├── operation_multith.go      多线程下载实现
├── operation_single.go       单线程下载实现
├── pool.go                   线程池
├── rate.go                   限流器
└── utils.go                  一些工具
```


## 💡 致谢

 - [Aria2](https://github.com/aria2/aria2)
