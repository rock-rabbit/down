package down

import (
	"bytes"
	"encoding/binary"
	"io"
)

// controlfile 控制文件，记录了断点下载所需要的信息
type controlfile struct {
	// varsion 2 字节 版本，当前版本只有 0（0x0000）
	varsion uint16

	// totalLength 8 字节 文件总长度
	totalLength uint64

	// completedLength 8 字节 已下载大小
	completedLength uint64

	// threadSize 4 字节 每个线程下载的大小
	threadSize uint32

	// threadNum 4 字节 未完成的的线程块数量
	threadNum uint32

	// threadblock 未完成的线程信息
	threadblock []*threadblock
}

// threadblock 未完成的线程信息
type threadblock struct {
	// completedLength 4 字节 已下载大小
	completedLength uint32
}

// newControlfile 创建一个固定大小的控制文件
func newControlfile(size int) *controlfile {
	threadblockTmp := make([]*threadblock, size)
	for i := 0; i < size; i++ {
		threadblockTmp[i] = new(threadblock)
	}
	return &controlfile{
		varsion:         0,
		totalLength:     0,
		completedLength: 0,
		threadSize:      0,
		threadNum:       uint32(size),
		threadblock:     threadblockTmp,
	}
}

// readControlfile 读取控制文件
func readControlfile(data []byte) *controlfile {
	dataLen := len(data)
	// 检查是否符合规范
	if dataLen < 26 || (dataLen-26)%4 != 0 {
		return nil
	}
	cf := newControlfile((dataLen - 26) / 4)
	binary.Read(bytes.NewReader(data[:2]), binary.BigEndian, &cf.varsion)
	binary.Read(bytes.NewReader(data[2:10]), binary.BigEndian, &cf.totalLength)
	binary.Read(bytes.NewReader(data[10:18]), binary.BigEndian, &cf.completedLength)
	binary.Read(bytes.NewReader(data[18:22]), binary.BigEndian, &cf.threadSize)
	binary.Read(bytes.NewReader(data[22:26]), binary.BigEndian, &cf.threadNum)
	b := 0
	for i := 26; i < dataLen-26; i += 4 {
		binary.Read(bytes.NewReader(data[i:i+4]), binary.BigEndian, &cf.threadblock[b].completedLength)
		b++
	}
	return cf
}

// Encoding 编码输出二进制
func (cf *controlfile) Encoding() *bytes.Buffer {
	buf := bytes.NewBuffer(make([]byte, 0, len(cf.threadblock)*8+26))
	binaryWrite := binaryWriteFunc(buf, binary.BigEndian)
	binaryWrite(cf.varsion)
	binaryWrite(cf.totalLength)
	binaryWrite(cf.completedLength)
	binaryWrite(cf.threadSize)
	binaryWrite(cf.threadNum)
	for _, v := range cf.threadblock {
		binaryWrite(v.completedLength)
	}
	return buf
}

func binaryWriteFunc(w io.Writer, order binary.ByteOrder) func(data any) error {
	return func(data any) error {
		return binary.Write(w, order, data)
	}
}
