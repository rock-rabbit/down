**🥳 当前项目处于<font color=red>开发阶段</font>, 请勿使用，可作为参考**

## 🎤 简介

零依赖，高性能，可扩展，结构清晰的 HTTP 文件下载器 Golang 包

## 🎉 功能
- HOOK
- 命令行进度条
- 多线程下载
- 单线程下载
- 覆盖下载
- 磁盘缓冲区

# 📝 进行中
- 断点下载

## 🏍️ 计划
- 限速下载
- 断点下载
- 文件自动重命名
- 多文件同时下载

´
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
	meta.Header.Set("referer", "http://www.68wu.cn/")
	// 给下载器添加进度条打印的 Hook
	down.Default.AddHook(down.DefaultBarHook)
	// 设置下载器的最大线程数，默认是 1
	down.Default.ThreadCount = 5
	// 执行下载, 你也可以使用 RunContext 传递一个 Context
	err := down.Default.Run(meta)
	if err != nil {
		panic(err)
	}
}
```
## 💡 致谢

 - [Aria2](https://github.com/aria2/aria2)
