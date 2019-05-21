package master

import (
	"context"
	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/clientopt"
	"github.com/mongodb/mongo-go-driver/mongo/findopt"
	"lurise/crontab/common"
	"time"
)

type LogMgr struct {
	Client     *mongo.Client
	Collection mongo.Collection
}

var (
	G_logMgr *LogMgr
)

func InitLogMgr() (err error) {
	var (
		client *mongo.Client
	)

	if client, err = mongo.Connect(context.TODO(),
		G_Config.MongodbUri,
		clientopt.ConnectTimeout(time.Duration(G_Config.MongodbConnectTimeout)*time.Millisecond)); err != nil {
		return
	}
	G_logMgr = &LogMgr{
		Client:     client,
		Collection: *client.Database("cron").Collection("log"),
	}
	return
}

func (logMgr *LogMgr) LogList(jobName string, limit int, skip int) (logArray []*common.JobLog, err error) {
	var (
		logFilter common.LogFilter
		sortOrder common.SortLogByStartTime
		cursor    mongo.Cursor

		jobLog *common.JobLog
	)

	logArray = make([]*common.JobLog, 0)
	//设定筛选条件
	logFilter = common.LogFilter{
		JobName: jobName,
	}
	//设定排序规则
	sortOrder = common.SortLogByStartTime{
		SortOrder: -1,
	}
	//根据条件查询日志列表
	if cursor, err = logMgr.Collection.Find(context.TODO(),
		logFilter,
		findopt.Skip(int64(skip)),
		findopt.Limit(int64(limit)),
		findopt.Sort(sortOrder)); err != nil {
		return
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		jobLog = &common.JobLog{}
		if err = cursor.Decode(jobLog); err != nil {
			return //有日志不合法
		}

		logArray = append(logArray, jobLog)
	}
	return
}
