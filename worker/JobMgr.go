package worker

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"lurise/crontab/common"
)

//全局单例变量
var G_jobMgr *JobMgr

type JobMgr struct {
	client  *clientv3.Client
	kv      clientv3.KV
	lease   clientv3.Lease
	watcher clientv3.Watcher
}

//定义Etcd的客户端
func InitJobMgr() (err error) {

	G_jobMgr = &JobMgr{
		client:  G_EtcdMgr.Client,
		kv:      G_EtcdMgr.Kv,
		lease:   G_EtcdMgr.Lease,
		watcher: G_EtcdMgr.Watcher,
	}
	//启动监听
	G_jobMgr.watchJobs()
	G_jobMgr.watchKiller()
	return
}

// 监听/cron/killer
func (jobMgr *JobMgr) watchKiller() (err error) {
	var (
		job *common.Job

		watchChan  clientv3.WatchChan
		watchResp  clientv3.WatchResponse
		watchEvent *clientv3.Event

		jobName string

		jobEvent *common.JobEvent
	)
	//获取现有的jobs

	//启动监听协程
	go func() {
		// 监听 /cron/killer/目录的变化
		watchChan = jobMgr.watcher.Watch(context.TODO(), common.JOB_KILLER_DIR, clientv3.WithPrefix())

		for watchResp = range watchChan {
			for _, watchEvent = range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT: //杀死任务事件
					jobName = common.ExtractKillerName(string(watchEvent.Kv.Key), common.DRONE_KILLER_DIR)
					job = &common.Job{
						Name: jobName,
					}
					//构建一个添加Event
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_KILLER, job)
					//将事件推送给Scheduler
					G_scheduler.PushJobEvent(jobEvent)
				case mvccpb.DELETE: //killer标记过期，自动删除
				}

			}
		}
	}()

	return
}

//监听/cron/jobs
func (jobMgr *JobMgr) watchJobs() (err error) {
	var (
		getResp *clientv3.GetResponse
		keyPair *mvccpb.KeyValue
		job     *common.Job
		jobName string

		watchChan          clientv3.WatchChan
		watchResp          clientv3.WatchResponse
		watchEvent         *clientv3.Event
		watchStartRevision int64

		jobEvent *common.JobEvent
	)
	//获取现有的jobs
	if getResp, err = jobMgr.kv.Get(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithPrefix()); err != nil {
		return
	}

	//将现存jobs同步给scheduler
	for _, keyPair = range getResp.Kvs {
		job = &common.Job{}
		if job, err = common.UnpackJob(keyPair.Value); err == nil {
			jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)
			G_scheduler.PushJobEvent(jobEvent)
		}
	}

	//启动监听协程
	go func() {
		watchStartRevision = getResp.Header.Revision + 1
		watchChan = jobMgr.watcher.Watch(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())

		//遍历监听channel
		for watchResp = range watchChan {
			for _, watchEvent = range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT:
					if job, err = common.UnpackJob(watchEvent.Kv.Value); err != nil {
						continue
					}
					//构建一个添加Event
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)
				case mvccpb.DELETE:
					// Delete /cron/jobs/job10
					jobName = common.ExtractJobName(string(watchEvent.Kv.Key))
					job = &common.Job{
						Name: jobName,
					}
					//构造一个删除Event
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_DELETE, job)
				}
				//将事件推送给Scheduler
				G_scheduler.PushJobEvent(jobEvent)
			}
		}
	}()

	return
}

//创建任务执行锁，分布式乐观锁
func (jobMgr *JobMgr) CreateJobLock(jobName string) (jobLock *Lock) {
	jobLock = InitLock(jobName, jobMgr.kv, jobMgr.lease)
	return
}
