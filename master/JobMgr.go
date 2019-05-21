package master

import (
	"context"
	"encoding/json"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"lurise/crontab/common"
	"time"
)

//全局单例变量
var G_jobMgr *JobMgr

type JobMgr struct {
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease
}

//定义Etcd的客户端
func InitJobMgr() (err error) {
	var (
		config clientv3.Config
		client *clientv3.Client
		kv     clientv3.KV
		lease  clientv3.Lease
	) //13658644481

	config = clientv3.Config{
		Endpoints:   G_Config.EtcdEndPoints,
		DialTimeout: time.Duration(G_Config.EtcdDailTimeout) * time.Millisecond,
	}

	if client, err = clientv3.New(config); err != nil {
		return
	}

	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)

	G_jobMgr = &JobMgr{
		client: client,
		kv:     kv,
		lease:  lease,
	}

	return
}

//向Etcd中保存job，保存路径为   /cron/jobs/任务名
func (jobMgr *JobMgr) SaveJob(job *common.Job) (oldJob *common.Job, err error) {
	var (
		jobKey   string
		jobValue []byte
		putResp  *clientv3.PutResponse
	)
	jobKey = common.JOB_SAVE_DIR + job.Name
	if jobValue, err = json.Marshal(job); err != nil {
		return
	}

	if putResp, err = jobMgr.kv.Put(context.TODO(), jobKey, string(jobValue), clientv3.WithPrevKV()); err != nil {
		return
	}

	if putResp.PrevKv != nil {
		if err = json.Unmarshal(putResp.PrevKv.Value, &oldJob); err != nil {
			//对于旧job反序列化失败并不会影响存新值，所以对err重新赋值，程序继续进行
			err = nil
			return
		}
	}
	return
}

//删除key为name的job
func (jobMgr *JobMgr) DeleteJob(name string) (oldJob *common.Job, err error) {
	var (
		keyname    string
		deleteResp *clientv3.DeleteResponse
	)
	keyname = common.JOB_SAVE_DIR + name
	if deleteResp, err = jobMgr.kv.Delete(context.TODO(), keyname, clientv3.WithPrevKV()); err != nil {
		return
	}

	if len(deleteResp.PrevKvs) != 0 {
		if err = json.Unmarshal([]byte(deleteResp.PrevKvs[0].Value), &oldJob); err != nil {
			err = nil
			return
		}
	}

	return
}

//获取job列表
func (jobMgr *JobMgr) JobList() (joblist []*common.Job, err error) {
	var (
		job     *common.Job
		getresp *clientv3.GetResponse
		kvPair  *mvccpb.KeyValue
	)
	if getresp, err = jobMgr.kv.Get(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithPrefix()); err != nil {
		return
	}

	joblist = make([]*common.Job, 0)
	for _, kvPair = range getresp.Kvs {
		//如果不新创建则会反序列化到空值
		job = &common.Job{}
		if err = json.Unmarshal(kvPair.Value, job); err != nil {
			err = nil
			continue
		}
		//因为数据可能空间不够用，会重新分配内存，所以需要重新赋值
		joblist = append(joblist, job)
	}

	return
}

//强杀job
func (jobMgr *JobMgr) JobKiller(name string) (err error) {
	var (
		leaseGrantResp *clientv3.LeaseGrantResponse
		leaseID        clientv3.LeaseID
	)

	if leaseGrantResp, err = jobMgr.lease.Grant(context.TODO(), 1); err != nil {
		return
	}

	leaseID = leaseGrantResp.ID

	if _, err = jobMgr.kv.Put(context.TODO(), common.JOB_KILLER_DIR+name, "", clientv3.WithLease(leaseID)); err != nil {
		return
	}

	return
}
