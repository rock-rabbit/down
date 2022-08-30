package down

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// operation 下载前的配置拷贝结构, 防止多线程使用时的配置变化
type operation struct {
	// config 下载配置
	config *Down

	// meta 下载信息
	meta []*Meta

	// od 操作下载
	od []*operatDown

	// hooks 通过 Down 的 PerHook 生成的 Hook
	hooks []Hook

	// filesize 文件总大小
	filesize int64

	// ctx 上下文
	ctx   context.Context
	close context.CancelFunc
}

// Stat 下载中发送给 Hook 的数据
type Stat struct {
	Meta []*Meta
	Down *Down
	// TotalLength 文件大小
	TotalLength int64
	// CompletedLength 已下载的文件大小
	CompletedLength int64
	// DownloadSpeed 每秒下载字节数
	DownloadSpeed int64
	// Connections 与资源服务器的连接数
	Connections int
}

func newOperation(ctx context.Context, down *Down, meta []*Meta) *operation {
	operat := new(operation)

	tmpod := make([]*operatDown, len(meta))
	for i := 0; i < len(meta); i++ {
		tmpod[i] = new(operatDown)
		tmpod[i].meta = meta[i]
		tmpod[i].config = down
	}
	operat.od = tmpod

	if down.Timeout != 0 {
		operat.ctx, operat.close = context.WithTimeout(ctx, down.Timeout)
	} else {
		operat.ctx, operat.close = context.WithCancel(ctx)
	}

	return operat
}

func (operat *operation) start() error {
	err := operat.initOD(operat.ctx)
	if err != nil {
		return err
	}
	operat.filesize = operat.getTotalLength()
	err = operat.makeHook()
	if err != nil {
		return err
	}

	go operat.sendStat(operat.getConnectCount)

	operat.startOD(operat.ctx)
	return nil
}

func (operat *operation) wait() error {
	var err error
	for _, v := range operat.od {
		err = v.wait()
		if err != nil {
			operat.finish(err)
			return err
		}
	}
	operat.finish(nil)
	return nil
}

func (operat *operation) finish(err error) {
	operat.close()

	operat.finishHook(err)
}

// makeHook 创建 Hook
func (operat *operation) makeHook() error {
	var err error
	operat.hooks = make([]Hook, len(operat.config.PerHooks))
	stat := &Stat{Down: operat.config, Meta: operat.meta, TotalLength: operat.filesize}
	for idx, perhook := range operat.config.PerHooks {
		operat.hooks[idx], err = perhook.Make(stat)
		if err != nil {
			return fmt.Errorf("Make Hook: %s", err)
		}
	}
	return nil
}

// finishHook 下载完成时通知 Hook
func (operat *operation) finishHook(down error) error {
	err := Hooks(operat.hooks).Finish(down, &Stat{
		Meta:        operat.meta,
		Down:        operat.config,
		TotalLength: operat.filesize,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "down error: finish hook failure: %v\n", err)
	}
	return nil
}

// sendHook 下载途中给 Hook 发送下载信息如 下载速度、已下载大小、下载连接数等...
func (operat *operation) sendHook(stat *Stat) error {
	err := Hooks(operat.hooks).Send(stat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "down error: send hook failure: %v\n", err)
	}
	return nil
}

// getCompletedLength 获取已下载文件大小
func (operat *operation) getCompletedLength() int64 {
	tmp := int64(0)
	for _, v := range operat.od {
		tmp += atomic.LoadInt64(v.cl)
	}
	return tmp
}

// getTotalLength 获取所有文件的总大小
func (operat *operation) getTotalLength() int64 {
	tmp := int64(0)
	for _, v := range operat.od {
		tmp += v.filesize
	}
	return tmp
}

// getConnectCount 获取连接数
func (operat *operation) getConnectCount() int {
	tmp := 0
	for _, v := range operat.od {
		tmp += v.wgpool.Count()
	}
	return tmp
}

// initOD 初始化
func (operat *operation) initOD(ctx context.Context) error {
	var err error
	for _, v := range operat.od {
		err = v.init(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// startOD 运行
func (operat *operation) startOD(ctx context.Context) {
	for _, v := range operat.od {
		v.start(ctx)
	}
}

// sendStat 下载资源途中对数据的处理和发送 Hook
func (operat *operation) sendStat(connectCountFunc func() int) {
	oldCompletedLength := operat.getCompletedLength()
	ratio := float64(time.Second) / float64(operat.config.SendTime)
	for {
		select {
		case <-time.After(operat.config.SendTime):
			connections := 1
			if connectCountFunc != nil {
				connections = connectCountFunc()
			}
			completedLength := operat.getCompletedLength()
			// 下载速度
			differ := completedLength - oldCompletedLength
			downloadSpeed := differ
			if operat.config.SendTime < time.Second || operat.config.SendTime > time.Second {
				downloadSpeed = int64(float64(completedLength-oldCompletedLength) * ratio)
			}
			oldCompletedLength = completedLength
			stat := &Stat{
				Meta:            operat.meta,
				Down:            operat.config,
				TotalLength:     operat.filesize,
				CompletedLength: completedLength,
				DownloadSpeed:   downloadSpeed,
				Connections:     connections,
			}
			operat.sendHook(stat)
		case <-operat.ctx.Done():
			return
		}
	}
}
