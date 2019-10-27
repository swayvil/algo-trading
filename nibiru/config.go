package nibiru

import (
	"encoding/json"
	"os"
	"sync"
)

const configFile string = "config.json"

type Config struct {
	WssURL  string `json:"wssURL"`
	BaseURL string `json:"baseURL"`
	Account struct {
		Secret     string `json:"secret"`
		Key        string `json:"key"`
		Passphrase string `json:"passphrase"`
	} `json:"account"`
	ElasticURL string `json:"elasticURL"`
	EsMatchIndex    string `json:"esMatchIndex"`
	EsFillIndex    string `json:"esFillIndex"`
	EsDiffSizeIndex    string `json:"esDiffSizeIndex"`
	EsSubSizeIndex    string `json:"esSubSizeIndex"`
	EsUser     string `json:"esUser"`
	EsPassword string `json:"esPassword"`
	Init       struct {
		Crypto   string `json:"crypto"`
		Currency string `json:"currency"`
		//NbMsgInit          int64   `json:"nbMsgInit"`
		Side               string  `json:"side"`
		LimitCashAvailable float64 `json:"limitCashAvailable"`
		LimitMinCash       float64 `json:"limitMinCash"`
		CashReserve        float64 `json:"cashReserve"`
		MinGains           float64 `json:"minGains"`
	} `json:"init"`
	Algo struct {
		PeriodShort int     `json:"periodShort"`
		PeriodLong  int     `json:"periodLong"`
		ThresholdShort      float64 `json:"thresholdShort"`
		ThresholdLong       float64 `json:"thresholdLong"`
	} `json:"algo"`
	ConsoleLog     string `json:"consoleLog"`
	OrdersBooksLog string `json:"ordersBooksLog"`
}

var instance *Config
var onceConfig sync.Once

func GetConfigInstance() *Config {
	onceConfig.Do(func() {
		instance = &Config{}
		instance.loadConfiguration(configFile)
	})
	return instance
}

func (config *Config) loadConfiguration(file string) {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		GetLoggerInstance().Error("In loadConfiguration: %s", err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
}
