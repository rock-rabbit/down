
## 🎤 简介

零依赖，高性能，可扩展，结构清晰的 HTTP 文件下载器 Golang 包

## 🎉 功能
- HOOK
- 命令行进度条 HOOK
- 多线程下载
- 单线程下载
- 覆盖下载
- 磁盘缓冲区
- 断点续传

## 📝 进行中
- 性能分析
- 性能调优

## 🏍️ 计划
- 限速下载
- 性能分析
- 文件自动重命名
- 多文件同时下载
- 生命周期 HOOK

## 🎊 安装
```bash
# github 安装
go get github.com/rock-rabbit/down
# gitee 安装
go get gitee.com/rock_rabbit/down
# 下载到本地使用，零依赖让这种方式变得极为方便
# ...
```
    
## 🪞 演示

![演示](https://www.68wu.cn/down/demonstration2.gif)
## 🛠 使用方法

``` golang
package main

import "github.com/rock-rabbit/down"

func main(){
	// 创建一个基本下载信息
	meta := down.NewMeta("http://downloadtest.kdatacenter.com/100MB", "./tmp", "")
	// 添加一个请求头
	meta.Header.Set("referer", "https://im.qq.com/")
	// down.Default 为默认配置的下载器, 你可以查看 Down 结构体配置自己的下载器
	down.Default.AddHook(down.DefaultBarHook)
	// down.Default.ThreadSize = 1024 << 10
	down.Default.ThreadCount = 1
	// 执行下载, 你也可以使用 RunContext 传递一个 Context
	path, err := down.Default.Run(meta)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("文件下载完成：" + path)
}
```

## 🔗 目录结构
```
.
├── LICENSE         开源协议 MIT
├── Makefile        快捷命令
├── README.md       说明文件
├── bar_hook.go     控制台进度条 Hook
├── controlfile.go  控制文件
├── down.go         下载器配置
├── go.mod
├── hook.go         定义 Hook 接口
├── meta.go         基本下载信息
├── mime.go         识别文件头
├── operation.go    具体的下载实现
├── pool.go         线程池
├── rate.go         限流器
├── request.go      网络请求
└── utils.go        一些工具
```


## 💡 致谢

 - [Aria2](https://github.com/aria2/aria2)
