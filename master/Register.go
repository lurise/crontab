package master

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"lurise/crontab/common"
	"time"
)

type MasterRegister struct {
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease

	localIP string
}

var (
	G_masterRegister *MasterRegister
)

func InitRegister() (err error) {
	//获取etcd不同的client后，可能会消除之前已经获取的client，在这里重复使用了G_jobMgr的客户端 TODO：将客户端独立出来，各部分取用
	var (
		localIp string
	)
	if localIp, err = common.GetLocalIP(); err != nil {
		return
	}

	G_masterRegister = &MasterRegister{
		client:  G_jobMgr.client,
		kv:      G_jobMgr.kv,
		lease:   G_jobMgr.lease,
		localIP: localIp,
	}

	G_masterRegister.keepOnLine()
	return
}

func (register *MasterRegister) keepOnLine() {
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

	regKey = common.MASTER_DIR

	if leaseGrantResp, err = register.lease.Grant(context.TODO(), 10); err != nil {
		goto RETRY
	}

	leaseID = leaseGrantResp.ID
	cancelCtx, cancelFunc = context.WithCancel(context.TODO())

	if leaseKeepAliveRespChan, err = register.lease.KeepAlive(cancelCtx, leaseID); err != nil {
		goto RETRY
	}

	if _, err = register.kv.Put(context.TODO(), regKey, register.localIP, clientv3.WithLease(leaseID)); err != nil {
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
}
