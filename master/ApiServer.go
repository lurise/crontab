package master

import (
	"encoding/json"
	"lurise/crontab/common"
	"net"
	"net/http"
	"strconv"
	"time"
)

type ApiServer struct {
	httpserver *http.Server
	listen     net.Listener
}

var (
	//单例对象
	G_apiServer *ApiServer
)

func InitApiServer() (err error) {
	var (
		mux           *http.ServeMux //路由对象
		listen        net.Listener
		httpServer    *http.Server
		dir           http.Dir
		staticHandler http.Handler
	)

	//r := gin.Default()
	//jobapi := r.Group("/job")
	//{
	//	jobapi.POST("/save", func(context *gin.Context) {
	//		w := context.Writer
	//		req := context.Request
	//
	//		handleJobSave(w, req)
	//	})
	//}

	//设置路由对象
	mux = http.NewServeMux()
	//设置路由规则
	mux.HandleFunc("/job/save", handleJobSave)
	mux.HandleFunc("/job/delete", handleJobDelete)
	mux.HandleFunc("/job/list", handleJobList)
	mux.HandleFunc("/job/kill", handleJobKiller)
	mux.HandleFunc("/job/log", handleJobLog)
	mux.HandleFunc("/worker/list", handleWorkerList)

	//设置静态页面
	dir = http.Dir("./webroot")
	staticHandler = http.FileServer(dir)
	mux.Handle("/", http.StripPrefix("/", staticHandler))

	//启动TCP监听
	if listen, err = net.Listen("tcp", ":"+strconv.Itoa(G_Config.ApiPoint)); err != nil {
		return
	}

	//配置http.Server对象
	httpServer = &http.Server{
		ReadTimeout:  time.Duration(G_Config.ReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(G_Config.WriteTimeout) * time.Millisecond,
		Handler:      mux,
	}

	//给服务端的单例赋值
	G_apiServer = &ApiServer{
		httpserver: httpServer,
		listen:     listen,
	}

	//go r.Run(":"+strconv.Itoa(G_Config.ApiPoint))
	//启动服务端
	go httpServer.Serve(listen)

	return nil
}

func handleWorkerList(w http.ResponseWriter, req *http.Request) {
	var (
		workerList []string
		err        error

		outBytes []byte
	)

	if workerList, err = G_WorkerMgr.GetWorkerList(); err != nil {
		goto ERR
	}
	if outBytes, err = common.BuildRespMsg(0, "success", workerList); err != nil {
		goto ERR
	}
	w.Write(outBytes)
	return
ERR:
	if outBytes, err = common.BuildRespMsg(-1, err.Error(), nil); err != nil {
		w.Write(outBytes)
		return
	}

}

//保存任务的接口
//Post方式提交：{"jobname":"job1","command":"echo hello","cronExpr":"* * * * *"}
func handleJobSave(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		postJob string
		job     common.Job
		bytes   []byte
		oldJob  *common.Job
	)

	//因解析会消耗cpu，go不会主动对form表单进行解析，所以需手动触发
	if err = r.ParseForm(); err != nil {
		goto ERR
	}
	//r.PostForm.Get("job")是post方法获取参数的方式
	postJob = r.PostForm.Get("job")

	//将json格式的job解析为job结构
	if err = json.Unmarshal([]byte(postJob), &job); err != nil {
		goto ERR
	}

	//保存job
	if oldJob, err = G_jobMgr.SaveJob(&job); err != nil {
		goto ERR
	}

	//构造返回结构
	if bytes, err = common.BuildRespMsg(0, "success", oldJob); err == nil {
		w.Write(bytes)
		return
	}
ERR:
	if bytes, err = common.BuildRespMsg(-1, err.Error(), nil); err == nil {
		w.Write(bytes)
	}
}

//删除接口
func handleJobDelete(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		jobName string

		respBytes []byte
		oldJob    *common.Job
	)

	if err = r.ParseForm(); err != nil {
		goto ERR
	}
	jobName = r.PostForm.Get("name")

	if oldJob, err = G_jobMgr.DeleteJob(jobName); err != nil {
		goto ERR
	}
	if respBytes, err = common.BuildRespMsg(0, "success", oldJob); err == nil {
		w.Write(respBytes)
		return
	}

ERR:
	respBytes, err = common.BuildRespMsg(-1, err.Error(), nil)
	w.Write(respBytes)
}

//任务列表接口
// get /job/list
func handleJobList(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		joblist []*common.Job

		respBytes []byte
	)

	if joblist, err = G_jobMgr.JobList(); err != nil {
		goto ERR
	}

	if respBytes, err = common.BuildRespMsg(0, "success", joblist); err == nil {
		w.Write(respBytes)
		return
	}
ERR:
	respBytes, err = common.BuildRespMsg(-1, err.Error(), nil)
	w.Write(respBytes)
}

//强杀job进程
//Post /job/killer {"name":"job1"}
func handleJobKiller(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		name      string
		respBytes []byte
	)
	if err = r.ParseForm(); err != nil {
		goto ERR
	}
	name = r.PostForm.Get("name")
	if err = G_jobMgr.JobKiller(name); err != nil {
		goto ERR
	}

	if respBytes, err = common.BuildRespMsg(0, "success", nil); err != nil {
		goto ERR
	}

	w.Write(respBytes)
	return
ERR:
	respBytes, err = common.BuildRespMsg(-1, err.Error(), nil)
	w.Write(respBytes)
}

//查询任务日志
func handleJobLog(w http.ResponseWriter, req *http.Request) {
	var (
		err error

		jobName    string
		skipParam  string
		limitParam string

		skip  int
		limit int

		logArray []*common.JobLog

		respBytes []byte
	)

	if err = req.ParseForm(); err != nil {
		goto ERR
	}

	jobName = req.Form.Get("name")
	skipParam = req.Form.Get("skip")
	limitParam = req.Form.Get("limit")

	if skip, err = strconv.Atoi(skipParam); err != nil {
		skip = 0
	}
	if limit, err = strconv.Atoi(limitParam); err != nil {
		limit = 20
	}

	if logArray, err = G_logMgr.LogList(jobName, limit, skip); err != nil {
		goto ERR
	}

	if respBytes, err = common.BuildRespMsg(0, "success", logArray); err != nil {
		goto ERR
	}
	w.Write(respBytes)
	return
ERR:
	respBytes, err = common.BuildRespMsg(-1, err.Error(), nil)
	w.Write(respBytes)
	return
}
