package down

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"io/fs"
	"os"
	"sync"
	"time"
)

const (
	// CONTROLFILESIZE 控制文件最小长度
	CONTROLFILESIZE = 34
	// THREADBLOCKSIZE 一个线程块的长度
	THREADBLOCKSIZE = 24
	// CONTROLFILEHEAD 控制文件头 100 111 119 110
	CONTROLFILEHEAD = "down"
)

// controlfile 控制文件，记录了断点下载所需要的信息
type controlfile struct {
	// varsion 2 字节 版本，当前版本只有 0（0x0000）
	varsion uint16

	// total 8 字节 文件总长度
	total int64

	// threadblock 未完成的线程信息
	threadblock []*threadblock
}

// threadblock 未完成的线程信息
type threadblock struct {
	// completed 8 字节 已下载大小
	completed int64
	// start 8 字节 开始字节
	start int64
	// end 8 字节 结束字节
	end int64
}

// operatCF 操作控制文件
type operatCF struct {
	ctx    context.Context
	path   string
	file   *os.File
	cf     *controlfile
	change bool
	mux    sync.Mutex
}

// newOperatCF 新建操控控制文件
func newOperatCF(ctx context.Context, ctlpath string) *operatCF {
	return &operatCF{
		ctx:  ctx,
		path: ctlpath,
		mux:  sync.Mutex{},
	}
}

// check 检查是否可以断点续传，控制文件损坏就删除
func (ocf *operatCF) check(perm fs.FileMode) (bool, error) {
	f, err := os.OpenFile(ocf.path, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return false, err
	}
	remove := func() error {
		err := f.Close()
		if err != nil {
			return err
		}
		err = os.Remove(ocf.path)
		if err != nil {
			return err
		}
		return nil
	}
	data, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		return false, err
	}
	cf := parseControlfile(data)
	if cf != nil {
		ocf.file = f
		ocf.cf = cf
	} else {
		return false, remove()
	}
	return true, nil
}

// addTreadblock 添加数据块
func (ocf *operatCF) addTreadblock(completed, start, end int64) int {
	if ocf.file == nil {
		return 0
	}
	ocf.mux.Lock()
	defer ocf.mux.Unlock()
	ocf.cf.threadblock = append(ocf.cf.threadblock, &threadblock{
		completed: completed,
		start:     start,
		end:       end,
	})
	ocf.change = true
	return len(ocf.cf.threadblock) - 1
}

// addCompleted 添加数据块已完成的数据量
func (ocf *operatCF) addCompleted(key int, completed int64) {
	if ocf.file == nil {
		return
	}
	ocf.mux.Lock()
	defer ocf.mux.Unlock()
	ocf.cf.threadblock[key].completed = completed
	ocf.change = true
}

// autoSave 自动保存控制文件
func (ocf *operatCF) autoSave(d time.Duration) {
	for {
		select {
		case <-time.After(d):
			if ocf.change {
				ocf.mux.Lock()
				ocf.save()
				ocf.change = false
				ocf.mux.Unlock()
			}
		case <-ocf.ctx.Done():
			return
		}
	}
}

// save 保存控制文件
func (ocf *operatCF) save() {
	if ocf.file == nil {
		return
	}
	// 防止系统崩溃导致的数据丢失，下载的文件需要强制刷入到磁盘
	ocf.file.Seek(0, 0)
	io.Copy(ocf.file, ocf.cf.encoding())
	// ocf.operatFile.file.Sync()
	ocf.file.Sync()
}

// remove 删除控制文件
func (ocf *operatCF) remove() {
	if ocf.file != nil {
		ocf.close()
		os.Remove(ocf.path)
	}
}

// open 打开文件
func (ocf *operatCF) open(perm fs.FileMode) error {
	f, err := os.OpenFile(ocf.path, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return err
	}
	ocf.file = f
	return nil
}

// close 关闭文件
func (ocf *operatCF) close() {
	if ocf.file != nil {
		ocf.file.Close()
	}
}

// completedLength 获取已下载的数据长度
func (cf *controlfile) completedLength() int64 {
	if len(cf.threadblock) == 0 {
		return 0
	}
	count := int64(0)
	for i := 0; i < len(cf.threadblock); i++ {
		count += int64(cf.threadblock[i].completed)
	}
	return count
}

// encoding 编码输出二进制
func (cf *controlfile) encoding() *bytes.Buffer {
	buf := bytes.NewBuffer(make([]byte, 0, len(cf.threadblock)*THREADBLOCKSIZE+CONTROLFILESIZE))
	binaryWrite := binaryWriteFunc(buf, binary.BigEndian)
	binaryWrite([]byte(CONTROLFILEHEAD))
	binaryWrite(cf.varsion)
	binaryWrite(cf.total)
	for _, v := range cf.threadblock {
		binaryWrite(v.completed)
		binaryWrite(v.start)
		binaryWrite(v.end)
	}
	return buf
}

func binaryWriteFunc(w io.Writer, order binary.ByteOrder) func(data any) error {
	return func(data any) error {
		return binary.Write(w, order, data)
	}
}

// newControlfile 创建一个固定大小的控制文件
func newControlfile(size int) *controlfile {
	threadblockTmp := make([]*threadblock, size)
	for i := 0; i < size; i++ {
		threadblockTmp[i] = new(threadblock)
	}
	return &controlfile{
		varsion:     0,
		total:       0,
		threadblock: threadblockTmp,
	}
}

// parseControlfile 解析控制文件
func parseControlfile(data []byte) *controlfile {
	dataLen := len(data)
	// 检查是否符合规范
	if dataLen < CONTROLFILESIZE || string(data[:4]) != CONTROLFILEHEAD || (dataLen-14)%THREADBLOCKSIZE != 0 {
		return nil
	}
	cf := newControlfile((dataLen - 14) / THREADBLOCKSIZE)
	binary.Read(bytes.NewReader(data[4:6]), binary.BigEndian, &cf.varsion)
	binary.Read(bytes.NewReader(data[6:14]), binary.BigEndian, &cf.total)
	b := 0
	for i := 14; i < dataLen-14; i += THREADBLOCKSIZE {
		binary.Read(bytes.NewReader(data[i:i+8]), binary.BigEndian, &cf.threadblock[b].completed)
		binary.Read(bytes.NewReader(data[i+8:i+16]), binary.BigEndian, &cf.threadblock[b].start)
		binary.Read(bytes.NewReader(data[i+16:i+24]), binary.BigEndian, &cf.threadblock[b].end)
		b++
	}
	return cf
}
