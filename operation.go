package down

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// operation 下载前的配置拷贝结构, 防止多线程使用时的配置变化
type operation struct {
	down *Down
	meta *Meta

	// hooks 通过 Down 的 PerHook 生成的 Hook
	hooks []Hook

	// client 通过 Down 配置生成 *http.Client
	client *http.Client

	// size 文件大小
	size int64

	// multithread 是否使用多线程下载
	multithread bool

	// breakpoint 是否使用断点续传
	breakpoint bool

	// filename 从 URI 和 头信息 中获得的文件名称, 未指定名称时使用
	filename string

	// outputPath 下载目标位置
	outputPath string

	// operatFile 操作文件
	operatFile *operatFile

	// controlfilePath 控制文件位置
	controlfilePath string
	operatCF        *operatCF

	// ctx 上下文
	ctx      context.Context
	ctxCance context.CancelFunc

	// done 下载完成
	done chan error
	err  error

	// stat 下载进行时的进度记录
	stat *stating
}

// Stat 下载中发送给 Hook 的数据
type Stat struct {
	Meta *Meta
	Down *Down
	// TotalLength 文件大小
	TotalLength int64
	// CompletedLength 已下载的文件大小
	CompletedLength int64
	// DownloadSpeed 每秒下载字节数
	DownloadSpeed int64
	// OutputPath 最终文件的位置
	OutputPath string
	// Connections 与资源服务器的连接数
	Connections int
}

// stating 下载进行时的数据
type stating struct {
	CompletedLength *int64
}

// start 开始执行下载
func (operat *operation) start() error {
	// 初始化操作
	if err := operat.init(); err != nil {
		return err
	}

	go operat.elected()

	return nil
}

// elected 下载方式判断
func (operat *operation) elected() {
	// 释放资源
	defer operat.ctxCance()
	defer operat.operatCF.close()
	defer operat.operatFile.close()

	// 单线程下载逻辑
	if !operat.multithread || operat.down.ThreadCount <= 1 {
		if operat.breakpoint {
			operat.singleBreakpoint()
		} else {
			operat.single()
		}
		return
	}

	// 多线程下载逻辑
	if operat.breakpoint {
		operat.multithBreakpoint()
	} else {
		operat.multith()
	}
}

// finish 下载完成
func (operat *operation) finish(err error) {
	if err == nil {
		// 发送成功 Hook
		operat.finishHook()
		// 删除控制文件
		operat.operatCF.remove()
	}
	operat.done <- err
}

// wait 等待下载完成
func (operat *operation) wait() error {
	return <-operat.done
}

// contextIsDone 判断 context 是否关闭
func (operat *operation) contextIsDone() bool {
	select {
	case <-operat.ctx.Done():
		return true
	default:
		return false
	}
}

// copyHooks 拷贝 Hook ，防止使用 Hook 中途发生变化
func (operat *operation) copyHooks() []Hook {
	var tmpHooks []Hook
	operat.down.mux.Lock()
	tmpHooks = make([]Hook, len(operat.hooks))
	copy(tmpHooks, operat.hooks)
	operat.down.mux.Unlock()
	return tmpHooks
}

// finishHook 下载完成时通知 Hook
func (operat *operation) finishHook() error {
	tmpHooks := operat.copyHooks()

	err := Hooks(tmpHooks).Finish(&Stat{
		Meta:        operat.meta,
		Down:        operat.down,
		TotalLength: operat.size,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "down error: finish hook failure: %v\n", err)
	}
	return nil
}

// sendHook 下载途中给 Hook 发送下载信息如 下载速度、已下载大小、下载连接数等...
func (operat *operation) sendHook(stat *Stat) error {
	tmpHooks := operat.copyHooks()

	err := Hooks(tmpHooks).Send(stat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "down error: send hook failure: %v\n", err)
	}
	return nil
}

// sendStat 下载资源途中对数据的处理和发送 Hook
func (operat *operation) sendStat(groupPool *WaitGroupPool) {
	oldCompletedLength := atomic.LoadInt64(operat.stat.CompletedLength)
	for {
		select {
		case <-time.After(time.Second):
			connections := 1
			if groupPool != nil {
				connections = groupPool.Count()
			}
			completedLength := atomic.LoadInt64(operat.stat.CompletedLength)
			downloadSpeed := completedLength - oldCompletedLength
			oldCompletedLength = completedLength
			stat := &Stat{
				Meta:            operat.meta,
				Down:            operat.down,
				TotalLength:     operat.size,
				CompletedLength: completedLength,
				DownloadSpeed:   downloadSpeed,
				Connections:     connections,
				OutputPath:      operat.outputPath,
			}
			operat.sendHook(stat)
		case <-operat.ctx.Done():
			return
		}
	}
}
