package down

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Down 下载器，请求配置和 Hook 信息
type Down struct {
	// perHooks 是返回下载进度的钩子，默认为空
	perHooks []PerHook
	// sendTime 给 Hook 发送下载进度的间隔时间，默认为 500ms
	sendTime time.Duration
	// threadCount 多线程下载时最多同时下载一个文件的最大线程，默认为 1
	threadCount int
	// threadSize 多线程下载时每个线程下载的大小，每个线程都会有一个自己下载大小的缓冲区，默认为 20M
	threadSize int
	// diskCache 磁盘缓冲区大小，默认为 16M
	diskCache int
	// speedLimit 下载速度限制，默认为 0 无限制
	speedLimit int
	// createDir 当需要创建目录时，是否创建目录，默认为 true
	createDir bool
	// allowOverwrite 是否允许覆盖文件，默认为 true
	allowOverwrite bool
	// autoFileRenaming 文件自动重命名，新文件名在名称之后扩展名之前加上一个点和一个数字（1..9999）。默认:true
	autoFileRenaming bool
	// continuew 是否启用断点续传，默认为 true
	continuew bool
	// autoSaveTnterval 自动保存控制文件的时间，默认为 1 秒
	autoSaveTnterval time.Duration
	// connectTimeout HTTP 连接请求的超时时间，默认为 5 秒
	connectTimeout time.Duration
	// timeout 下载总超时时间，默认为 10 分钟
	timeout time.Duration
	// retryNumber 最多重试次数，默认为 5
	retryNumber int
	// retryTime 重试时的间隔时间，默认为 0
	retryTime time.Duration
	// proxy Http 代理设置，默认为 http.ProxyFromEnvironment
	proxy func(*http.Request) (*url.URL, error)
	// tempFileExt 临时文件后缀, 默认为 down
	tempFileExt string
	// mux 锁
	mux sync.Mutex
}

var (
	// std 默认下载器
	std = New()

	// Error 自定义错误
	ErrorDefault       = "down error: %v"
	ErrorFileExist     = "已存在文件 %s，若允许替换文件请将 down.AllowOverwrite 设为 true"
	ErrorRequestStatus = "%s HTTP Status Code %d"
	ErrInvalidWrite    = errors.New("invalid write result")
)

// New 创建一个默认的下载器
func New() *Down {
	return &Down{
		perHooks:         make([]PerHook, 0),
		sendTime:         time.Millisecond * 500,
		threadCount:      1,
		threadSize:       20971520,
		diskCache:        16777216,
		speedLimit:       0,
		createDir:        true,
		allowOverwrite:   true,
		continuew:        true,
		autoFileRenaming: true,
		autoSaveTnterval: time.Second * 1,
		connectTimeout:   time.Second * 5,
		timeout:          time.Minute * 10,
		retryNumber:      5,
		retryTime:        0,
		proxy:            http.ProxyFromEnvironment,
		tempFileExt:      "down",
		mux:              sync.Mutex{},
	}
}

// Run 运行下载，接收三个参数: 下载链接、输出目录、输出文件名
func (down *Down) Run(s ...string) (string, error) {
	return down.RunContext(context.Background(), s...)
}

// RunContext 基于 Context 运行下载，接收三个参数: 下载链接、输出目录、输出文件名
func (down *Down) RunContext(ctx context.Context, s ...string) (string, error) {
	if len(s) == 0 {
		return "", fmt.Errorf(ErrorDefault, "下载参数不能为空")
	}
	return down.runContext(ctx, SimpleMeta(s...))
}

// Start 非阻塞运行下载
func (down *Down) Start(s ...string) (*Operation, error) {
	return down.StartContext(context.Background(), s...)

}

// StartContext 基于 Context 非阻塞运行下载
func (down *Down) StartContext(ctx context.Context, s ...string) (*Operation, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf(ErrorDefault, "下载参数不能为空")
	}
	return down.startContext(ctx, SimpleMeta(s...))
}

// RunMerging 合并下载
// uri 包含下载链接和输出文件名的数组
// outpath 输出目录
func (down *Down) RunMerging(uri [][2]string, outpath string) ([]string, error) {
	return down.RunMergingContext(context.Background(), uri, outpath)
}

// RunMergingContext 基于 context 合并下载
// uri 包含下载链接和输出文件名的数组
// outpath 输出目录
func (down *Down) RunMergingContext(ctx context.Context, uri [][2]string, outpath string) ([]string, error) {
	if len(uri) == 0 {
		return []string{}, fmt.Errorf(ErrorDefault, "下载参数不能为空")
	}

	tmpMeta := make([]*Meta, len(uri))
	for i := 0; i < len(uri); i++ {
		tmpMeta[i] = NewMeta(uri[i][0], outpath, uri[i][1])
	}

	return down.runMergingContext(ctx, tmpMeta)
}

// RunMeta 自己创建下载信息执行下载
func (down *Down) RunMeta(meta *Meta) (string, error) {
	return down.runContext(context.Background(), meta)
}

// RunMetaContext 基于 context 自己创建下载信息执行下载
func (down *Down) RunMetaContext(ctx context.Context, meta *Meta) (string, error) {
	return down.runContext(ctx, meta)
}

// StartMeta 非阻塞运行下载
func (down *Down) StartMeta(meta *Meta) (*Operation, error) {
	return down.StartMetaContext(context.Background(), meta)

}

// StartMetaContext 基于 Context 非阻塞运行下载
func (down *Down) StartMetaContext(ctx context.Context, meta *Meta) (*Operation, error) {
	return down.startContext(ctx, meta)
}

// RunMergingMeta 自己创建下载信息合并下载
func (down *Down) RunMergingMeta(meta []*Meta) ([]string, error) {
	return down.runMergingContext(context.Background(), meta)
}

func (down *Down) RunMergingMetaContext(ctx context.Context, meta []*Meta) ([]string, error) {
	return down.runMergingContext(ctx, meta)
}

// SetSendTime 设置给 Hook 发送下载进度的间隔时间
func (down *Down) SetSendTime(n time.Duration) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.sendTime = n
}

// SetSpeedLimit 设置限速，每秒下载字节
func (down *Down) SetSpeedLimit(n int) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.speedLimit = n
}

// SetThreadCount 设置多线程时的最大线程数
func (down *Down) SetThreadCount(n int) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.threadCount = n
}

// SetThreadCount 设置多线程时每个线程下载的最大长度
func (down *Down) SetThreadSize(n int) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.threadSize = n
}

// SetDiskCache 设置磁盘缓冲区大小
func (down *Down) SetDiskCache(n int) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.diskCache = n
}

// SetDiskCache 设置当需要创建目录时，是否创建目录
func (down *Down) SetCreateDir(n bool) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.createDir = n
}

// SetAllowOverwrite 设置是否允许覆盖文件
func (down *Down) SetAllowOverwrite(n bool) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.allowOverwrite = n
}

// SetContinue 设置是否启用断点续传
func (down *Down) SetContinue(n bool) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.continuew = n
}

// SetAutoSaveTnterval 设置自动保存控制文件的时间
func (down *Down) SetAutoSaveTnterval(n time.Duration) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.autoSaveTnterval = n
}

// SetConnectTimeout 设置 HTTP 连接请求的超时时间
func (down *Down) SetConnectTimeout(n time.Duration) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.connectTimeout = n
}

// SetTimeout 设置下载总超时时间
func (down *Down) SetTimeout(n time.Duration) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.timeout = n
}

// SetRetryNumber 设置下载最多重试次数
func (down *Down) SetRetryNumber(n int) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.retryNumber = n
}

// SetRetryTime 重试时的间隔时间
func (down *Down) SetRetryTime(n time.Duration) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.retryTime = n
}

// SetProxy 设置 Http 代理
func (down *Down) SetProxy(n func(*http.Request) (*url.URL, error)) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.proxy = n
}

// SetTempFileExt 设置临时文件后缀
func (down *Down) SetTempFileExt(n string) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.tempFileExt = n
}

// Copy 在执行下载前，会拷贝 Down
func (down *Down) Copy() *Down {
	down.mux.Lock()
	defer down.mux.Unlock()

	tmpDown := *down
	tmpDown.perHooks = make([]PerHook, len(down.perHooks))
	copy(tmpDown.perHooks, down.perHooks)

	tmpDown.mux = sync.Mutex{}
	return &tmpDown
}

// AddHook 添加 Hook 的创建接口
func (down *Down) AddHook(perhook PerHook) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.perHooks = append(down.perHooks, perhook)
}

// RunContext 基于 context 执行下载，阻塞等待完成
func (down *Down) runContext(ctx context.Context, meta *Meta) (string, error) {
	outpath, err := down.runMergingContext(ctx, []*Meta{meta})
	if err != nil {
		return "", err
	}
	if len(outpath) > 0 {
		return outpath[0], err
	}
	return "", nil
}

// StartContext 基于 context 非阻塞运行
func (down *Down) startContext(ctx context.Context, meta *Meta) (*Operation, error) {
	return down.mergingStartContext(ctx, []*Meta{meta})
}

// RunMerging 基于 context 合并下载，阻塞运行
func (down *Down) runMergingContext(ctx context.Context, meta []*Meta) ([]string, error) {
	var (
		err    error
		operat *Operation
	)
	operat, err = down.mergingStartContext(ctx, meta)
	if err != nil {
		return []string{}, err
	}
	return operat.Wait()
}

// MergingStartContext 基于 context 合并下载，非阻塞运行
func (down *Down) mergingStartContext(ctx context.Context, meta []*Meta) (*Operation, error) {
	operat := down.operation(ctx, meta)
	if err := operat.start(); err != nil {
		return nil, fmt.Errorf(ErrorDefault, err)
	}
	return &Operation{operat: operat}, nil
}

// Operation 包装 operation
type Operation struct {
	operat *operation
}

// Wait 阻塞等待完成
func (o *Operation) Wait() ([]string, error) {
	err := o.operat.wait()
	if err != nil {
		return o.operat.getOutpath(), fmt.Errorf(ErrorDefault, err)
	}
	return o.operat.getOutpath(), nil
}

// operation 创建 operation
func (down *Down) operation(ctx context.Context, meta []*Meta) *operation {
	var operat *operation
	tmpMeta := make([]*Meta, len(meta))
	for i := 0; i < len(meta); i++ {
		tmpMeta[i] = meta[i].Copy()
	}
	// 组合操作结构,将配置拷贝一份
	operat = newOperation(ctx, down.Copy(), tmpMeta)
	return operat
}
