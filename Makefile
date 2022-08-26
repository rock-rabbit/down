GO ?= go

# test 单元测试 和 代码覆盖率
.PHONY: test
test:
	$(GO) test -v -coverprofile=./cover.out
	$(GO) tool cover -html=./cover.out -o ./coverage.html
	open ./coverage.html
 
 # TestDown 测试下载
 .PHONY: TestDown
 TestDown:
	$(GO) test -run ^TestDown$ -v -count=1