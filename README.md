
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
- 多文件同时下载

## 📝 进行中
- 写文档

## 🏍️ 计划
- 限速下载
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
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("文件下载完成：", path)
}
// 默认输出目录为 ./，运行后输出:
// 文件下载完成：/Users/rockrabbit/projects/down/tmp/100MB
```
简单的使用命令行进度条 Hook
``` golang
	// 给默认下载器添加进度条 Hook，这是一个全局操作
	down.AddHook(down.DefaultBarHook)

	path, err := down.Run("http://downloadtest.kdatacenter.com/100MB")
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("文件下载完成：", path)
// 运行后输出:
// 100.00 MB / 100.00 MB [================================] 100% 12.06 MB/s 0s CN:1
// 文件下载完成：/Users/rockrabbit/projects/down/tmp/down0.bin
```
简单的多文件同时下载
``` golang
	// 给默认下载器添加进度条 Hook，这是一个全局操作
	down.AddHook(down.DefaultBarHook)

	metaMerging := [][2]string{
		{"http://downloadtest.kdatacenter.com/100MB", "down1.bin"},
		{"http://downloadtest.kdatacenter.com/100MB", "down2.bin"},
	}
	path, err := down.RunMerging(metaMerging, "./")
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("文件下载完成：", path)
// 运行后输出:
// 200.00 MB / 200.00 MB [================================] 100% 12.06 MB/s 0s CN:2
// 文件下载完成：[/Users/rockrabbit/projects/down/tmp/down1.bin /Users/rockrabbit/projects/down/tmp/down2.bin]
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
