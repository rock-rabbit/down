package down

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"
)

// mimeType 常见类型文件头
var mimeType = map[string]string{
	/* 图片类型 */
	"ffd8ffe0": "jpg",
	"ffd8ffe1": "jpg",
	"ffd8ffe8": "jpg",
	"47494638": "gif",
	"49492a00": "tif",
	"424d":     "bmp",
	"41433130": "dwg",
	"38425053": "psd",
	/* 音频类型 */
	"2e7261fd": "ram",
	"57415645": "wav",
	"4d546864": "mid",
	"494433":   "mp3",
	"fffb50":   "mp3",
	/* 视频类型 */
	"41564920":                 "avi",
	"2e524d46":                 "rm",
	"000001ba":                 "mpg",
	"000001b3":                 "mpg",
	"6d6f6f76":                 "mov",
	"6d646174":                 "mov",
	"000000186674797033677035": "mp4",
	"3026b2758e66cf11":         "asf",
	/* 压缩类型 */
	"504b0304": "zip",
	"52617221": "rar",
	/* 其他类型 */
	"7b5c727466":                   "rtf",
	"3c3f786d6c":                   "xml",
	"68746d6c3e":                   "html",
	"44656c69766572792d646174653a": "eml",
	"cfad12fec5fd746f":             "dbx",
	"2142444e":                     "pst",
	"5374616e64617264204a":         "mdb",
	"ff575043":                     "wpd",
	"252150532d41646f6265":         "ps",
	"255044462d312e":               "pdf",
	"ac9ebd8f":                     "qdf",
	"e3828596":                     "pwl",
}

func getFileType(src []byte) string {
	if len(src) == 0 {
		return ""
	}
	var fileType string
	headBytes := bytesToHexString(src)
	for k, v := range mimeType {
		if strings.HasPrefix(headBytes, k) {
			fileType = v
			break
		}
	}
	return fileType
}

func bytesToHexString(src []byte) string {
	res := bytes.Buffer{}
	if src == nil || len(src) <= 0 {
		return ""
	}
	temp := make([]byte, 0)
	i, length := 100, len(src)
	if length < i {
		i = length
	}
	for j := 0; j < i; j++ {
		sub := src[j] & 0xFF
		hv := hex.EncodeToString(append(temp, sub))
		if len(hv) < 2 {
			res.WriteString(strconv.FormatInt(int64(0), 10))
		}
		res.WriteString(hv)
	}
	return res.String()
}
