package down

import (
	"context"
	"io/fs"
	"os"
)

// operatFile 操作文件
type operatFile struct {
	ctx      context.Context
	operatCF *operatCF
	file     *os.File
	bufsize  int
}

// operatFileAt 指定位置
type operatFileAt struct {
	of        *operatFile
	id        int
	start     int64
	completed int64
}

// newOperatFile 创建操作文件
func newOperatFile(ctx context.Context, path string, perm fs.FileMode, bufsize int) (*operatFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return nil, err
	}
	return &operatFile{ctx: ctx, file: f, bufsize: bufsize}, nil
}

// close 关闭文件
func (of *operatFile) close() {
	if of.file != nil {
		of.close()
	}
}

// makeFileAt 创建文件位置的操作文件
func (of *operatFile) makeFileAt(id int, start int64) *operatFileAt {
	return &operatFileAt{id: id, of: of, start: start}
}

// Write 写入
func (ofat *operatFileAt) Write(p []byte) (n int, err error) {
	n, err = ofat.of.file.WriteAt(p, ofat.start)
	if err != nil {
		return n, err
	}
	err = ofat.of.file.Sync()
	ofat.start += int64(n)
	ofat.completed += int64(n)
	// 更新操作文件
	ofat.of.operatCF.setTB(ofat.id, ofat.completed)
	return
}
