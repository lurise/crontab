package worker

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	//etcd配置
	EtcdEndPoints   []string `json:"etcdEndpoints"`
	EtcdDailTimeout int      `json:"etcdDailTimeout"`

	//mongodb配置
	MongodbUri            string `json:"mongodbUri"`
	MongodbConnectTimeout int    `json:"mongodbConnectionTimeout"`

	LogBatchSize int `json:"logBatchSize"`

	JobLogCommitTimeout int `json:"jobLogCommitTimeout"`

	ContainerStopTimeout int `json:"containerStopTimeout"`

	DroneImageTag      string   `json:"droneImageTag"`
	DroneImagePullAddr string   `json:"droneImagePullAddr"`
	DroneRunningPorts  []string `json:"droneRunningPorts"`
}

var (
	G_Config Config
)

func InitConfig(fileName string) (err error) {
	var (
		content []byte
		config  Config
	)
	if content, err = ioutil.ReadFile(fileName); err != nil {
		return
	}

	if err = json.Unmarshal(content, &config); err != nil {
		return
	}

	G_Config = config
	return
}
