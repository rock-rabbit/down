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
	// total 4 字节 总长度
	total int32
	// completed 4 字节 已下载大小
	completed int32
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
func newOperatCF(ctx context.Context) *operatCF {
	return &operatCF{
		ctx: ctx,
		mux: sync.Mutex{},
	}
}

// newControlfile 创建控制文件
func (ocf *operatCF) newControlfile() {
	ocf.cf = newControlfile(0)
}

// addTB 添加数据块
func (ocf *operatCF) addTB(total, completed int32, start, end int64) {
	ocf.mux.Lock()
	defer ocf.mux.Unlock()
	ocf.cf.threadblock = append(ocf.cf.threadblock, &threadblock{
		total:     total,
		completed: completed,
		start:     start,
		end:       end,
	})
	ocf.change = true
}

// setTB 设置数据块
func (ocf *operatCF) setTB(key int, completed int32) {
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
	ocf.file.Seek(0, 0)
	io.Copy(ocf.file, ocf.cf.Encoding())
	ocf.file.Sync()
}

// getCF 获取控制文件
func (ocf *operatCF) getCF() *controlfile {
	return ocf.cf
}

// remove 删除控制文件
func (ocf *operatCF) remove() {
	if ocf.file != nil {
		ocf.close()
		os.Remove(ocf.path)
	}
}

// open 打开文件
func (ocf *operatCF) open(path string, perm fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return err
	}
	ocf.file = f
	ocf.path = path
	return nil
}

// read 读取文件
func (ocf *operatCF) read() error {
	data, err := io.ReadAll(ocf.file)
	if err != nil {
		return err
	}
	cf := ParseControlfile(data)
	if cf != nil {
		ocf.cf = cf
	}
	return nil
}

// close 关闭文件
func (ocf *operatCF) close() {
	if ocf.file != nil {
		ocf.file.Close()
	}
}

// CompletedLength 获取已下载的数据长度
func (cf *controlfile) CompletedLength() int64 {
	if len(cf.threadblock) == 0 {
		return 0
	}
	count := int64(0)
	for i := 0; i < len(cf.threadblock); i++ {
		count += int64(cf.threadblock[i].completed)
	}
	return count
}

// Encoding 编码输出二进制
func (cf *controlfile) Encoding() *bytes.Buffer {
	buf := bytes.NewBuffer(make([]byte, 0, len(cf.threadblock)*THREADBLOCKSIZE+CONTROLFILESIZE))
	binaryWrite := binaryWriteFunc(buf, binary.BigEndian)
	binaryWrite([]byte(CONTROLFILEHEAD))
	binaryWrite(cf.varsion)
	binaryWrite(cf.total)
	for _, v := range cf.threadblock {
		binaryWrite(v.total)
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

// ParseControlfile 解析控制文件
func ParseControlfile(data []byte) *controlfile {
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
		binary.Read(bytes.NewReader(data[i:i+4]), binary.BigEndian, &cf.threadblock[b].total)
		binary.Read(bytes.NewReader(data[i:i+8]), binary.BigEndian, &cf.threadblock[b].completed)
		binary.Read(bytes.NewReader(data[i:i+16]), binary.BigEndian, &cf.threadblock[b].start)
		binary.Read(bytes.NewReader(data[i:i+24]), binary.BigEndian, &cf.threadblock[b].end)
		b++
	}
	return cf
}
