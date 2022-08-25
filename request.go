package down

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// rangeDo 基于 range 的请求
func (operat *operation) rangeDo(start, end int64) (*http.Response, error) {
	res, err := operat.defaultDo(func(req *http.Request) error {
		req.Header.Set("range", fmt.Sprintf("bytes=%d-%d", start, end))
		return nil
	})
	if err != nil {
		return res, fmt.Errorf("request Do: %s", err)
	}
	return res, nil
}

// defaultDo 基于默认参数的请求
func (operat *operation) defaultDo(call func(req *http.Request) error) (*http.Response, error) {
	req, err := operat.request(http.MethodGet, operat.meta.URI, operat.meta.Body)
	if err != nil {
		return nil, fmt.Errorf("request: %s", err)
	}
	if call != nil {
		err = call(req)
		if err != nil {
			return nil, err
		}
	}
	res, err := operat.do(req)
	if err != nil {
		return res, fmt.Errorf("request Do: %s", err)
	}
	return res, nil
}

// request 对于 http.NewRequestWithContext 的包装
func (operat *operation) request(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(operat.ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	header := make(http.Header, len(operat.meta.Header))

	for k, v := range operat.meta.Header {
		tmpVal := make([]string, len(v))
		copy(tmpVal, v)
		header[k] = v
	}

	req.Header = header

	return req, nil
}

// do 对于 client.Do 的包装，主要实现重试机制
func (operat *operation) do(rsequest *http.Request) (*http.Response, error) {
	// 请求失败时，重试机制
	var (
		res          *http.Response
		requestError error
		retryNum     = 0
	)
	for ; ; retryNum++ {
		res, requestError = operat.client.Do(rsequest)
		if requestError == nil && res.StatusCode < 400 {
			break
		} else if retryNum+1 >= operat.down.RetryNumber {
			var err error
			if requestError != nil {
				err = fmt.Errorf("down error: %v", requestError)
			} else {
				err = fmt.Errorf("down error: %s HTTP %d", operat.meta.URI, res.StatusCode)
			}
			return nil, err
		}
		time.Sleep(operat.down.RetryTime)
	}
	return res, nil

}
