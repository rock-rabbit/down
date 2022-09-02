package down

import (
	"bufio"
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

	// rate 限速器
	rate *Limiter

	// speedLimit
	speedLimit int

	// ctx 上下文
	ctx context.Context
}

// newOperatFile 创建操作文件
func newOperatFile(ctx context.Context, operatCF *operatCF, outpath string, cl *int64, bufsize int, perm fs.FileMode, speedLimit int) (*operatFile, error) {
	f, err := os.OpenFile(outpath, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return nil, err
	}
	var rate *Limiter
	if speedLimit != 0 {
		rate = NewLimiter(Limit(speedLimit), speedLimit)
	}
	return &operatFile{ctx: ctx, file: f, bufsize: bufsize, cl: cl, operatCF: operatCF, speedLimit: speedLimit, rate: rate}, nil
}

// makeFileAt 创建文件位置的操作文件
func (of *operatFile) makeFileAt(id int, start int64) *operatFileAt {
	return &operatFileAt{id: id, of: of, start: start}
}

// iocopy 数据拷贝
func (of *operatFile) iocopy(src io.Reader, start int64, blockid, dataSize int) error {
	// 硬盘缓冲区大小
	writeBufsize := of.bufsize
	if writeBufsize > dataSize {
		writeBufsize = dataSize
	}
	// 新建硬盘写入
	dst := bufio.NewWriterSize(of.makeFileAt(blockid, start), writeBufsize)

	var (
		err     error
		written int64
		readbuf []byte
	)

	// 读缓冲大小
	defRadesize := 32 * 1024
	if defRadesize > dataSize {
		defRadesize = dataSize
	}
	// 限速下载时的读缓冲大小
	if of.rate != nil && of.speedLimit < defRadesize {
		readbuf = make([]byte, of.speedLimit)
	} else {
		readbuf = make([]byte, defRadesize)
	}

	for {
		var (
			nr int
			er error
		)
		if of.rate == nil {
			nr, er = src.Read(readbuf)
		} else {
			nr, er = of.rateRead(src, readbuf)
		}
		if nr > 0 {
			nw, ew := dst.Write(readbuf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = ErrInvalidWrite
				}
			}
			of.addcl(nw)
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	if err != nil {
		return err
	}
	// 写入完毕，将写缓冲区的内容写入到文件
	err = dst.Flush()
	if err != nil {
		return err
	}
	return nil
}

// rateRead 限速读取
func (of *operatFile) rateRead(src io.Reader, buf []byte) (n int, err error) {
	n, err = src.Read(buf)
	if err != nil {
		return
	}
	err = of.rate.WaitN(of.ctx, n)

	return
}

// addcl 新增进度
func (of *operatFile) addcl(n int) {
	atomic.AddInt64(of.cl, int64(n))
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
