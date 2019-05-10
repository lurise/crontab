package main

import (
	"lurise/crontab/master"
	"runtime"
)

func initEnv() {
	//设置go的最大协程与所在运行主机的cpu核数相同
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	//设置环境变量
	initEnv()

	//启动服务端
	if err := master.InitApiServer(); err != nil {

	}
}
