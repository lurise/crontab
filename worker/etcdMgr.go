package worker

import (
	"github.com/coreos/etcd/clientv3"
	"time"
)

type EtcdMgr struct {
	Client  *clientv3.Client
	Kv      clientv3.KV
	Lease   clientv3.Lease
	Watcher clientv3.Watcher
}

var (
	G_EtcdMgr *EtcdMgr
)

func InitEtcdMgr() (err error) {
	var (
		config  clientv3.Config
		client  *clientv3.Client
		kv      clientv3.KV
		lease   clientv3.Lease
		watcher clientv3.Watcher
	)

	config = clientv3.Config{
		Endpoints:   G_Config.EtcdEndPoints,
		DialTimeout: time.Duration(G_Config.EtcdDailTimeout) * time.Millisecond,
	}
	if client, err = clientv3.New(config); err != nil {
		return
	}

	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)
	watcher = clientv3.NewWatcher(client)

	G_EtcdMgr = &EtcdMgr{
		Client:  client,
		Kv:      kv,
		Lease:   lease,
		Watcher: watcher,
	}
	return
}
