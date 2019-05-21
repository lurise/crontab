package master

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	ApiPoint     int `json:"apiPort"`
	ReadTimeout  int `json:"apiReadTimeout"`
	WriteTimeout int `json:"apiWriteTimeout"`

	//etcd配置
	EtcdEndPoints   []string `json:"etcdEndpoints"`
	EtcdDailTimeout int      `json:"etcdDailTimeout"`

	//Mongo配置
	MongodbUri            string `json:"mongodbUri"`
	MongodbConnectTimeout int    `json:"mongodbConnectionTimeout"`
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
