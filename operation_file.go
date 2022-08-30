package down

import (
	"context"
	"io"
	"io/fs"
	"os"
	"sync/atomic"
)

/*
operatFile 对于下载文件的控制
*/

// operatFile 操作文件
type operatFile struct {
	// operatCF 控制文件
	operatCF *operatCF

	// file 当前下载的文件控制
	file *os.File

	// bufsize 磁盘缓冲区大小
	bufsize int

	// cl 文件总体下载进度
	cl *int64

	// ctx 上下文
	ctx context.Context
}

// newOperatFile 创建操作文件
func newOperatFile(ctx context.Context, operatCF *operatCF, outpath string, cl *int64, bufsize int, perm fs.FileMode) (*operatFile, error) {
	f, err := os.OpenFile(outpath, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return nil, err
	}
	return &operatFile{ctx: ctx, file: f, bufsize: bufsize, cl: cl}, nil
}

// makeFileAt 创建文件位置的操作文件
func (of *operatFile) makeFileAt(id int, start int64) *operatFileAt {
	return &operatFileAt{id: id, of: of, start: start}
}

// iocopy 数据拷贝
func (of *operatFile) iocopy(src io.Reader, start int64, blockid, dataSize int) error {
	// 硬盘缓冲区大小
	bufSize := of.bufsize
	if bufSize > dataSize {
		bufSize = dataSize
	}
	// 新建硬盘缓冲区写入
	ofat := of.makeFileAt(blockid, start)
	readerSend := func(n int) {
		atomic.AddInt64(of.cl, int64(n))
	}
	_, err := io.CopyBuffer(ofat, &ioProxyReader{reader: src, send: readerSend}, make([]byte, bufSize))
	if err != nil {
		return err
	}
	return nil
}

// close 关闭文件
func (of *operatFile) close() {
	of.operatCF.close()
	if of.file != nil {
		of.file.Close()
	}
}

// operatFileAt 指定位置
type operatFileAt struct {
	// of 下载文件控制
	of *operatFile

	// id 数据块ID
	id int

	// start 写入指针
	start int64
}

// Write 写入
func (ofat *operatFileAt) Write(p []byte) (n int, err error) {
	n, err = ofat.of.file.WriteAt(p, ofat.start)
	if err != nil {
		return n, err
	}

	ofat.start += int64(n)
	// 更新操作文件
	ofat.of.operatCF.addCompleted(ofat.id, int64(n))
	return
}
