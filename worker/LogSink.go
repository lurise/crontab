package worker

import (
	"context"
	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/clientopt"
	"lurise/crontab/common"
	"time"
)

type logSink struct {
	Client        *mongo.Client
	logCollection *mongo.Collection

	logChan        chan *common.JobLog   //日志处理channel，用于添加日志队列，并判断日志队列是否已满足提交要求
	autoCommitChan chan *common.LogBatch //日志处理超时channel，用于判断队列在指定时间内是否已满，如果超时，则自动提交
}

var (
	G_logSink *logSink
)

func InitLogSink() (err error) {
	var (
		client *mongo.Client
	)

	if client, err = mongo.Connect(
		context.TODO(),
		G_Config.MongodbUri,
		clientopt.ConnectTimeout(time.Duration(
			G_Config.MongodbConnectTimeout)*time.Millisecond)); err != nil {
		return
	}
	G_logSink = &logSink{
		Client:        client,
		logCollection: client.Database("cron").Collection("log"),

		//日志处理channel，用于添加日志队列，并判断日志队列是否已满足提交要求
		logChan: make(chan *common.JobLog),
		//日志处理超时channel，用于判断队列在指定时间内是否已满，如果超时，则自动提交
		autoCommitChan: make(chan *common.LogBatch),
	}
	go G_logSink.writeLoop()
	return
}

//日志存储协程
func (logSink *logSink) writeLoop() {
	var (
		log      *common.JobLog
		logBatch *common.LogBatch

		commitTimer     *time.Timer
		logTimeOutBatch *common.LogBatch
	)

	for {
		select {
		case log = <-logSink.logChan:
			//将这条log写入mongodb中，因为每次插入需要等待mongodb的一次网络往返，耗时可能会花费比较多，需要按批次插入
			if logBatch == nil {
				logBatch = &common.LogBatch{}
				commitTimer = time.AfterFunc(
					time.Duration(G_Config.JobLogCommitTimeout)*time.Millisecond,
					//闭包函数，用以锁定batch的上下文
					func(batch *common.LogBatch) func() {
						//发出超时通知，避免writeLoop与timer同时操作---串行操作
						return func() {
							//不直接提交mongo，而是重新调用writeLoop中的方法，避免多线程操作
							logSink.autoCommitChan <- batch
						}
					}(logBatch))
			}

			//将新日志追加到批次中
			logBatch.Logs = append(logBatch.Logs, log)

			//如果日志已经达到预定要求数量，则将日志批量插入到mongo中
			if len(logBatch.Logs) >= G_Config.LogBatchSize {
				//储存日志信息
				logSink.saveLogs(logBatch)
				//清空日志队列
				logBatch = nil
			}

		//过期的批次
		case logTimeOutBatch = <-logSink.autoCommitChan:
			//判断logBatch是否与当前Batch相等,借以判断logBatch是否已经在预定时间内提交
			if logTimeOutBatch != logBatch {
				continue //跳过已经被提交的批次
			}

			//将批次写入到mongo中
			logSink.saveLogs(logTimeOutBatch)
			//清空logBatch
			logBatch = nil
			//取消定时器
			commitTimer.Stop()
		}
	}
}

func (logSink *logSink) saveLogs(logs *common.LogBatch) {
	logSink.logCollection.InsertMany(context.TODO(), logs.Logs)
}

func (logSink *logSink) Append(jobLog *common.JobLog) {
	select {
	case logSink.logChan <- jobLog:
	default:
		//队列满了就丢弃
	}

}
