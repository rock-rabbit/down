package down

import (
	"context"
	"io/fs"
	"os"
	"sync/atomic"
)

// operatFile 操作文件
type operatFile struct {
	ctx     context.Context
	file    *os.File
	cl      *int64
	bufsize int
}

// operatFileAt 指定位置
type operatFileAt struct {
	start int64
	of    *operatFile
}

// newOperatFile 创建操作文件
func newOperatFile(ctx context.Context, path string, perm fs.FileMode, bufsize int) (*operatFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return nil, err
	}
	return &operatFile{ctx: ctx, file: f, bufsize: bufsize}, nil
}

func (ofat *operatFileAt) Write(p []byte) (n int, err error) {
	n = len(p)

	atomic.AddInt64(ofat.of.cl, int64(n))

	return
}

// makeFileAt 创建文件位置的操作文件
func (of *operatFile) makeFileAt(start int64) *operatFileAt {
	return &operatFileAt{of: of, start: start}
}

func (of *operatFile) close() {
	if of.file != nil {
		of.close()
	}
}
