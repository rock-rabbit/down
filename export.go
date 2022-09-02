package down

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Run 运行下载，接收三个参数: 下载链接、输出目录、输出文件名
func Run(s ...string) (string, error) {
	return std.Run(s...)
}

// RunContext 基于 Context 运行下载，接收三个参数: 下载链接、输出目录、输出文件名
func RunContext(ctx context.Context, s ...string) (string, error) {
	return std.RunContext(ctx, s...)
}

// Start 非阻塞运行下载
func Start(s ...string) (*Operation, error) {
	return std.Start(s...)

}

// StartContext 基于 Context 非阻塞运行下载
func StartContext(ctx context.Context, s ...string) (*Operation, error) {
	return std.StartContext(ctx, s...)
}

// RunMerging 合并下载
// uri 包含下载链接和输出文件名的数组
// outpath 输出目录
func RunMerging(uri [][2]string, outpath string) ([]string, error) {
	return std.RunMerging(uri, outpath)
}

// RunMergingContext 基于 context 合并下载
// uri 包含下载链接和输出文件名的数组
// outpath 输出目录
func RunMergingContext(ctx context.Context, uri [][2]string, outpath string) ([]string, error) {
	return std.RunMergingContext(ctx, uri, outpath)
}

// RunMeta 自己创建下载信息执行下载
func RunMeta(meta *Meta) (string, error) {
	return std.RunMeta(meta)
}

// RunMetaContext 基于 context 自己创建下载信息执行下载
func RunMetaContext(ctx context.Context, meta *Meta) (string, error) {
	return std.RunMetaContext(ctx, meta)
}

// StartMeta 非阻塞运行下载
func StartMeta(meta *Meta) (*Operation, error) {
	return std.StartMeta(meta)

}

// StartMetaContext 基于 Context 非阻塞运行下载
func StartMetaContext(ctx context.Context, meta *Meta) (*Operation, error) {
	return std.StartMetaContext(ctx, meta)
}

// RunMergingMeta 自己创建下载信息合并下载
func RunMergingMeta(meta []*Meta) ([]string, error) {
	return std.RunMergingMeta(meta)
}

// RunMergingMetaContext 基于 Context 自己创建下载信息合并下载
func RunMergingMetaContext(ctx context.Context, meta []*Meta) ([]string, error) {
	return std.RunMergingMetaContext(ctx, meta)
}

// SetSendTime 设置给 Hook 发送下载进度的间隔时间
func SetSendTime(n time.Duration) {
	std.SetSendTime(n)
}

// SetThreadCount 设置多线程时的最大线程数
func SetThreadCount(n int) {
	std.SetThreadCount(n)
}

// SetThreadCount 设置多线程时每个线程下载的最大长度
func SetThreadSize(n int) {
	std.SetThreadSize(n)
}

// SetDiskCache 设置磁盘缓冲区大小
func SetDiskCache(n int) {
	std.SetDiskCache(n)
}

// SetDiskCache 设置当需要创建目录时，是否创建目录
func SetCreateDir(n bool) {
	std.SetCreateDir(n)
}

// SetAllowOverwrite 设置是否允许覆盖文件
func SetAllowOverwrite(n bool) {
	std.SetAllowOverwrite(n)
}

// SetContinue 设置是否启用断点续传
func SetContinue(n bool) {
	std.SetContinue(n)
}

// SetSpeedLimit 设置限速，每秒下载字节
func SetSpeedLimit(n int) {
	std.SetSpeedLimit(n)
}

// SetAutoSaveTnterval 设置自动保存控制文件的时间
func SetAutoSaveTnterval(n time.Duration) {
	std.SetAutoSaveTnterval(n)
}

// SetConnectTimeout 设置 HTTP 连接请求的超时时间
func SetConnectTimeout(n time.Duration) {
	std.SetConnectTimeout(n)
}

// SetTimeout 设置下载总超时时间
func SetTimeout(n time.Duration) {
	std.SetTimeout(n)
}

// SetRetryNumber 设置下载最多重试次数
func SetRetryNumber(n int) {
	std.SetRetryNumber(n)
}

// SetRetryTime 重试时的间隔时间
func SetRetryTime(n time.Duration) {
	std.SetRetryTime(n)
}

// SetProxy 设置 Http 代理
func SetProxy(n func(*http.Request) (*url.URL, error)) {
	std.SetProxy(n)
}

// SetTempFileExt 设置临时文件后缀
func SetTempFileExt(n string) {
	std.SetTempFileExt(n)
}

// Copy 在执行下载前，会拷贝 Down
func Copy() *Down {
	return std.Copy()
}

// AddHook 添加 Hook 的创建接口
func AddHook(perhook PerHook) {
	std.AddHook(perhook)
}
