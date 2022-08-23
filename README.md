**🥳 当前项目处于<font color=red>开发阶段</font>, 请勿使用**

## 🎤 简介
零依赖，高性能，可扩展，结构清晰的 HTTP 文件下载器 Golang 包

## 🎊 安装
``` bash
# github 安装
go get github.com/rock-rabbit/down
# gitee 安装
go get gitee.com/rock_rabbit/down
# 下载到本地使用，零依赖让这种方式变得极为方便
# ...
```

## 🎉 功能
- 命令行进度条
- 多线程下载
- 单线程下载
- 覆盖下载
- HOOK

## 🏍️ 计划
- 限速下载
- 断点下载

## 使用方式
``` golang
	// 创建一个基本下载信息
	meta := down.NewMeta("http://downloadtest.kdatacenter.com/100MB", "./tmp", "")
	// 添加一个请求头
	meta.Header.Set("referer", "http://www.68wu.cn/")
	// down.Default 为默认配置的下载器, 你可以查看 Down 结构体配置自己的下载器
	// Down 和 Mata 结构体可复用, 多线程安全
	// 给下载器添加进度条打印的 Hook
	down.Default.AddHook(down.DefaultBarHook)
	// 执行下载, 你也可以使用 RunContext 传递一个 Context
	err := down.Default.Run(meta)
	if err != nil {
		log.Panic(err)
	}
```
介绍两个主要的结构体 Down 和 Meta
``` golang

type Meta struct {
	URI        string
    // OutputName 输出文件名称, 为空时自动获取
	OutputName string
    // OutputDir 输出目录
	OutputDir  string
    // Header 请求时的 Header
	Header     http.Header
	// Perm 新建文件的权限, 默认为 0600
	Perm fs.FileMode
}

type Down struct {
	// PerHooks 是返回下载进度的钩子
	PerHooks []PerHook
	// ThreadCount 多线程下载时最多同时下载一个文件的线程
	ThreadCount int
	// ThreadSize 多线程下载时每个线程下载的字节数
	ThreadSize int64
	// Replace 遇到相同文件时是否要强制替换
	Replace bool
	// Resume 是否每次都重新下载,不尝试断点续传
	Resume bool
	// ConnectTimeout HTTP 连接请求的超时时间
	ConnectTimeout time.Duration
	// Timeout 超时时间
	Timeout time.Duration
	// RetryNumber 最多重试次数
	RetryNumber int
	// RetryTime 重试时的间隔时间
	RetryTime time.Duration
	// Proxy Http 代理设置
	Proxy func(*http.Request) (*url.URL, error)
	// TempFileExt 临时文件后缀, 默认为 down
	TempFileExt string
	// mux 锁
	mux sync.Mutex
}
```

Hook, 具体 Hook 的实现请查看 bar_hook.go 文件实现的进度条 hook
``` golang
// PerHook 是用来创建 Hook 的接口
// down 会在下载之前执行 Make 获得 Hook
// PerHook 的存在是为了在每次执行下载时获取新的 Hook, 不然所有下载都会共用一个 Hook
type PerHook interface {
	Make(stat *Stat) (Hook, error)
}

type Hook interface {
	Send(*Stat) error
	Finish(*Stat) error
}
```