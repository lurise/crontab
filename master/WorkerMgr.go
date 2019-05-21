package master

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"lurise/crontab/common"
)

type WorkerMgr struct {
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease
}

var (
	G_WorkerMgr *WorkerMgr
)

func InitWorkerMgr() (err error) {
	var (
	//config clientv3.Config
	//client *clientv3.Client
	//kv     clientv3.KV
	//lease  clientv3.Lease
	)

	//config = clientv3.Config{
	//	Endpoints:
	//	G_Config.EtcdEndPoints,
	//	DialTimeout: time.Duration(G_Config.EtcdDailTimeout) * time.Millisecond,
	//}
	//
	//if client, err = clientv3.New(config); err != nil {
	//	return
	//}
	//kv = clientv3.NewKV(client)
	//lease = clientv3.NewLease(client)

	G_WorkerMgr = &WorkerMgr{
		client: G_jobMgr.client,
		kv:     G_jobMgr.kv,
		lease:  G_jobMgr.lease,
	}
	return
}

func (workerMgr *WorkerMgr) GetWorkerList() (workerList []string, err error) {
	var (
		workerKey   string
		etcdGetResp *clientv3.GetResponse

		kvPair *mvccpb.KeyValue
	)
	workerList = make([]string, 0)

	workerKey = common.WORKER_DIR
	if etcdGetResp, err = G_WorkerMgr.kv.Get(context.TODO(), workerKey, clientv3.WithPrefix()); err != nil {
		return
	}

	for _, kvPair = range etcdGetResp.Kvs {
		workerList = append(workerList, common.ExtractWorkerAddr(string(kvPair.Key)))
	}

	return
}
