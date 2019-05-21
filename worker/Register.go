package worker

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"lurise/crontab/common"
	"time"
)

type Register struct {
	client  *clientv3.Client
	kv      clientv3.KV
	lease   clientv3.Lease
	watcher clientv3.Watcher

	localIP  string
	masterIP string

	masterAvailable bool //判断主节点是否可用
}

var (
	G_Register *Register
)

func InitRegister() (err error) {
	var (
		localIp string
	)

	if localIp, err = common.GetLocalIP(); err != nil {
		return
	}

	G_Register = &Register{
		client:  G_EtcdMgr.Client,
		kv:      G_EtcdMgr.Kv,
		lease:   G_EtcdMgr.Lease,
		localIP: localIp,
		watcher: G_EtcdMgr.Watcher,
	}
	if err = G_Register.GetMasterIP(); err != nil {
		return
	}
	G_Register.WatchMasterIPChange()
	G_Register.keepOnLine()
	return

}

func (register *Register) GetMasterIP() (err error) {
	var (
		getResp  *clientv3.GetResponse
		masterIP string
	)
	if getResp, err = register.kv.Get(context.TODO(), common.MASTER_DIR); err != nil {
		register.masterAvailable = false
		return
	}
	masterIP = string(getResp.Kvs[0].Value)
	register.masterIP = masterIP
	register.masterAvailable = true
	return
}

func (register *Register) WatchMasterIPChange() {
	go func() {
		var (
			watchChan  clientv3.WatchChan
			watchResp  clientv3.WatchResponse
			watchEvent *clientv3.Event
		)
		watchChan = register.watcher.Watch(context.TODO(), common.MASTER_DIR)

		for watchResp = range watchChan {
			for _, watchEvent = range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT:
					register.GetMasterIP() //重新获取IP   TODO:对于正在注册中的drone该如何处理？
				case mvccpb.DELETE:
					register.masterIP = ""
					register.masterAvailable = false //主节点宕机，无法使用主节点
				}
			}
		}
	}()
}

func (register *Register) keepOnLine() {
	go func() {
		var (
			regKey                 string
			leaseGrantResp         *clientv3.LeaseGrantResponse
			leaseKeepAliveRespChan <-chan *clientv3.LeaseKeepAliveResponse
			leaseKeepAliveResp     *clientv3.LeaseKeepAliveResponse
			leaseID                clientv3.LeaseID

			cancelCtx  context.Context
			cancelFunc context.CancelFunc

			err error
		)
		cancelFunc = nil

		regKey = common.WORKER_DIR + register.localIP

		if leaseGrantResp, err = register.lease.Grant(context.TODO(), 10); err != nil {
			goto RETRY
		}

		leaseID = leaseGrantResp.ID
		cancelCtx, cancelFunc = context.WithCancel(context.TODO())

		if leaseKeepAliveRespChan, err = register.lease.KeepAlive(cancelCtx, leaseID); err != nil {
			goto RETRY
		}

		if _, err = register.kv.Put(context.TODO(), regKey, "", clientv3.WithLease(leaseID)); err != nil {
			goto RETRY
		}

		for {
			select {
			case leaseKeepAliveResp = <-leaseKeepAliveRespChan:
				if leaseKeepAliveResp == nil {
					goto RETRY
				}
			}
		}
	RETRY:
		time.Sleep(1 * time.Second)
		if cancelFunc != nil {
			cancelFunc()
		}
	}()

}
