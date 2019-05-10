package master

import (
	"net"
	"net/http"
	"time"
)

type ApiServer struct {
	httpserver *http.Server
}

var (
	//单例对象
	G_apiServer *ApiServer
)

//保存任务的接口
func handleJobSave(w http.ResponseWriter, r *http.Request) {

}

func InitApiServer() (err error) {
	var (
		mux        *http.ServeMux //路由对象
		listen     net.Listener
		httpServer *http.Server
	)

	//设置路由对象
	mux = http.NewServeMux()
	//设置路由规则
	mux.HandleFunc("/job/save", handleJobSave)

	//启动TCP监听
	if listen, err = net.Listen("tcp", ":8070"); err != nil {
		return err
	}

	//配置http.Server对象
	httpServer = &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      mux,
	}

	//给服务端的单例赋值
	G_apiServer = &ApiServer{
		httpserver: httpServer,
	}

	//启动服务端
	go httpServer.Serve(listen)

	return nil
}
