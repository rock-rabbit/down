package down

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Down 下载器，请求配置和 Hook 信息
type Down struct {
	// PerHooks 是返回下载进度的钩子，默认为空
	PerHooks []PerHook
	// SendTime 给 Hook 发送下载进度的间隔时间，默认为 500ms
	SendTime time.Duration
	// ThreadCount 多线程下载时最多同时下载一个文件的最大线程，默认为 1
	ThreadCount int
	// ThreadSize 多线程下载时每个线程下载的大小，每个线程都会有一个自己下载大小的缓冲区，默认为 20M
	ThreadSize int
	// DiskCache 磁盘缓冲区大小，这只在一个线程下载时有用，默认为 16M
	DiskCache int
	// SpeedLimit 下载速度限制，默认为 0 无限制
	SpeedLimit int
	// CreateDir 当需要创建目录时，是否创建目录，默认为 true
	CreateDir bool
	// AllowOverwrite 是否允许覆盖文件，默认为 true
	AllowOverwrite bool
	// AutoFileRenaming 文件自动重命名，新文件名在名称之后扩展名之前加上一个点和一个数字（1..9999）。默认:true
	AutoFileRenaming bool
	// Continue 是否启用断点下载，默认为 true
	Continue bool
	// AutoSaveTnterval 自动保存控制文件的时间，默认为 1 秒
	AutoSaveTnterval time.Duration
	// ConnectTimeout HTTP 连接请求的超时时间，默认为 5 秒
	ConnectTimeout time.Duration
	// Timeout 下载总超时时间，默认为 10 分钟
	Timeout time.Duration
	// RetryNumber 最多重试次数，默认为 5
	RetryNumber int
	// RetryTime 重试时的间隔时间，默认为 0
	RetryTime time.Duration
	// Proxy Http 代理设置，默认为 http.ProxyFromEnvironment
	Proxy func(*http.Request) (*url.URL, error)
	// TempFileExt 临时文件后缀, 默认为 down
	TempFileExt string
	// mux 锁
	mux sync.Mutex
}

var (
	// Default 默认下载器
	Default = New()

	// Error 自定义错误
	ErrorFileExist = "已存在文件 %s，若允许替换文件请将 down.AllowOverwrite 设为 true"
)

// New 创建一个默认的下载器
func New() *Down {
	return &Down{
		PerHooks:         make([]PerHook, 0),
		SendTime:         time.Millisecond * 500,
		ThreadCount:      1,
		ThreadSize:       20971520,
		DiskCache:        16777216,
		SpeedLimit:       0,
		CreateDir:        true,
		AllowOverwrite:   true,
		Continue:         true,
		AutoFileRenaming: true,
		AutoSaveTnterval: time.Second * 1,
		ConnectTimeout:   time.Second * 5,
		Timeout:          time.Minute * 10,
		RetryNumber:      5,
		RetryTime:        0,
		Proxy:            http.ProxyFromEnvironment,
		TempFileExt:      "down",
		mux:              sync.Mutex{},
	}
}

// copy 在执行下载前，会拷贝 Down
func (down *Down) copy() *Down {
	tmpDown := *down
	tmpDown.PerHooks = make([]PerHook, len(down.PerHooks))
	copy(tmpDown.PerHooks, down.PerHooks)

	tmpDown.mux = sync.Mutex{}
	return &tmpDown
}

// AddHook 添加 Hook 的创建接口
func (down *Down) AddHook(perhook PerHook) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.PerHooks = append(down.PerHooks, perhook)
}

// Run 执行下载，阻塞等待完成
func (down *Down) Run(meta *Meta) error {
	return down.RunContext(context.Background(), meta)
}

// RunContext 基于 context 执行下载，阻塞等待完成
func (down *Down) RunContext(ctx context.Context, meta *Meta) error {
	var (
		err    error
		operat *Operation
	)
	operat, err = down.StartContext(ctx, meta)
	if err != nil {
		return err
	}
	err = operat.Wait()
	if err != nil {
		return err
	}
	return nil
}

// Start 非阻塞运行
func (down *Down) Start(meta *Meta) (*Operation, error) {
	return down.StartContext(context.Background(), meta)
}

// StartContext 基于 context 非阻塞运行
func (down *Down) StartContext(ctx context.Context, meta *Meta) (*Operation, error) {
	operat := down.operation(ctx, meta)
	if err := operat.start(); err != nil {
		return nil, fmt.Errorf("down error: %s", err)
	}
	return &Operation{operat: operat}, nil
}

// Operation 包装 operation
type Operation struct {
	operat *operation
}

// Wait 阻塞等待完成
func (o *Operation) Wait() error {
	return o.operat.wait()
}

// operation 创建 operation
func (down *Down) operation(ctx context.Context, meta *Meta) *operation {
	var operat *operation
	// 组合操作结构,将配置拷贝一份
	down.mux.Lock()
	operat = &operation{
		down: down.copy(),
		meta: meta.copy(),
		stat: &stating{
			CompletedLength: new(int64),
		},
		ctx:  ctx,
		done: make(chan error),
	}
	down.mux.Unlock()
	return operat
}
