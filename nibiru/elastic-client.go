package nibiru

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

const ES_TYPE string = "order"
const REQUEST_TIMEOUT int = 10 // in seconds

type Order struct {
	MatchTime time.Time `json:"matchTime"`
	ProductId string    `json:"product_id"`
	Size      float64   `json:"size"`
	Price     float64   `json:"price"`
	Side      string    `json:"side"`
}

type ElasticClient struct {
	elasticURL   string
	httpClient   *http.Client
	esMatchIndex string
	esFillIndex  string
	esDiffSizeIndex  string
	esSubSizeIndex string
	esType       string
	esUser       string
	esPassword   string
}

func NewElasticClient() *ElasticClient {
	var httpClient = &http.Client{Timeout: time.Duration(REQUEST_TIMEOUT) * time.Second}
	return &ElasticClient{GetConfigInstance().ElasticURL, httpClient, GetConfigInstance().EsMatchIndex, GetConfigInstance().EsFillIndex, GetConfigInstance().EsDiffSizeIndex, GetConfigInstance().EsSubSizeIndex, ES_TYPE, GetConfigInstance().EsUser, GetConfigInstance().EsPassword}
}

func (elasticClient *ElasticClient) operation(ope string, requestBody string, resource string) string {
	re := regexp.MustCompile("\\s") // remove all spaces
	requestBody = re.ReplaceAllString(requestBody, "")

	u, _ := url.ParseRequestURI(elasticClient.elasticURL)
	u.Path = resource
	urlStr := u.String()

	req, err := http.NewRequest(ope, urlStr, bytes.NewBufferString(requestBody))
	if elasticClient.esUser != "" {
		req.SetBasicAuth(elasticClient.esUser, elasticClient.esPassword)
	}

	if err != nil {
		GetLoggerInstance().Error("In elastic-client/%s. Request failed: %s", ope, err.Error())
		os.Exit(1)
	}
	resp, _ := elasticClient.httpClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode >= 200 || resp.StatusCode < 300 { // OK
		bodyBytes, err2 := ioutil.ReadAll(resp.Body)
		if err2 != nil {
			GetLoggerInstance().Error("In elastic-client/%s. Failed reading response: %s", ope, err2.Error())
			os.Exit(1)
		} else {
			return string(bodyBytes)
		}
	} else {
		GetLoggerInstance().Error("In elastic-client/%s. Status code KO: %d", ope, resp.StatusCode)
		os.Exit(1)
	}
	return ""
}

func (elasticClient *ElasticClient) Aggregate(field string, intervalMinutes int, aggFunction string, side string) float64 {
	t := time.Now().Add(time.Duration(intervalMinutes) * time.Minute * -1).Format(time.RFC3339) // now - intervalMinutes
	var index = elasticClient.esMatchIndex

	requestBody := `{
	    "aggs" : {
	        "price_ranges" : {
	            "range" : {
	                "field" : "matchTime",
	                "ranges" : [
	                    { "from" : "` + t + `" }
	                ]
	            },
			    "aggs" : {
			        "result" : { "` + aggFunction + `" : { "field" : "` + field + `" } }
			    }
	        }
	    }
	}`

	if side != "" {
		index += "_" + side
	}
	resource := "/" + index + "/_search"
	resp := elasticClient.operation("GET", requestBody, resource)

	if resp == "" {
		GetLoggerInstance().Error("In elastic-client/Aggregate. Response NIL")
		os.Exit(1)
	}
	//GetLoggerInstance().Info("In elastic-client/Aggregate. Response: %s", resp)
	esResponse := &ESResponse{}
	err := json.Unmarshal([]byte(resp), esResponse)
	if err != nil {
		GetLoggerInstance().Error("In elastic-client/Aggregate. Failed unmarshaling response: %s", err.Error())
		os.Exit(1)
	}
	return esResponse.Aggregations.PriceRanges.Buckets[0].Result.Value
}

func (elasticClient *ElasticClient) IndexOrder(matchTime time.Time, productId string, size float64, price float64, side string) {
	t := matchTime.Format(time.RFC3339)
	var index = elasticClient.esMatchIndex
	requestBody := `{
				"matchTime": "` + t + `",
				"product_id": "` + productId + `",
				"size": "` + strconv.FormatFloat(size, 'E', -1, 64) + `",
				"price": "` + strconv.FormatFloat(price, 'E', -1, 64) + `"`
	if side != "" {
		requestBody += `, "side": "` + side + `"`
		index += "_" + side
	}
	requestBody += `}`
	
	resource := "/" + index + "/orders"
	//GetLoggerInstance().Error("In elastic-client/IndexOrder - size: %f, price: %f ", size, price)
	resp := elasticClient.operation("POST", requestBody, resource)

	if resp == "" {
		GetLoggerInstance().Error("In elastic-client/IndexOrder. Response NIL")
		os.Exit(1)
	}
}

func (elasticClient *ElasticClient) GetLatestPrice() float64 {
	requestBody := `{
	  "size": 1,
	  "sort": [
	    {
	      "matchTime": {
	        "order": "desc"
	      }
	    }
	  ]
	}`
	resource := "/" + elasticClient.esMatchIndex + "/orders/_search"
	resp := elasticClient.operation("GET", requestBody, resource)

	if resp == "" {
		GetLoggerInstance().Error("In elastic-client/GetLatestRecord. Response NIL")
		os.Exit(1)
	}
	esResponse := &ESResponse{}
	err := json.Unmarshal([]byte(resp), esResponse)
	if err != nil {
		GetLoggerInstance().Error("In elastic-client/GetLatestRecord. Failed unmarshaling response: %s", err.Error())
		os.Exit(1)
	}
	return esResponse.Hits.Hits[0].Source.Price
}

func (elasticClient *ElasticClient) IndexFillOrder(fillTime time.Time, productId string, size float64, price float64, side string) {
	t := fillTime.Format(time.RFC3339)
	requestBody := `{
				"fillTime": "` + t + `",
				"product_id": "` + productId + `",
				"size": "` + strconv.FormatFloat(size, 'E', -1, 64) + `",
				"price": "` + strconv.FormatFloat(price, 'E', -1, 64) + `",
				"side": "` + side + `"
				}`
	resource := "/" + elasticClient.esFillIndex + "/orders"
	//GetLoggerInstance().Error("In elastic-client/IndexOrder - size: %f, price: %f ", size, price)
	resp := elasticClient.operation("POST", requestBody, resource)

	if resp == "" {
		GetLoggerInstance().Error("In elastic-client/IndexFillOrder. Response NIL")
		os.Exit(1)
	}
}

func (elasticClient *ElasticClient) IndexDiffSize(t time.Time, productId string, sizeSell float64, sizeBuy float64, price float64) {
	sizeSellByBuy := sizeSell / sizeBuy
	sizeBuyBySell := sizeBuy / sizeSell
	GetLoggerInstance().Info("Algo/Run - IndexDiffSize - sizeSellByBuy: %f, sizeBuyBySell: %f", sizeSellByBuy, sizeBuyBySell)
	GetLoggerInstance().Info("Algo/Run - IndexDiffSize - sizeSellByBuy: %f", sizeSellByBuy)
	GetLoggerInstance().Info("Algo/Run - IndexDiffSize - sizeSellByBuy: %s", strconv.FormatFloat(sizeSellByBuy, 'E', -1, 64) )
	GetLoggerInstance().Info("Algo/Run - IndexDiffSize - sizeBuyBySellStr: %s", strconv.FormatFloat(sizeBuyBySell, 'E', -1, 64))
	requestBody := `{
				"time": "` + t.Format(time.RFC3339) + `",
				"product_id": "` + productId + `",
				"diff_size_sell_by_buy": "` + strconv.FormatFloat(sizeSellByBuy, 'E', -1, 64) + `",
				"diff_size_buy_by_sell": "` + strconv.FormatFloat(sizeBuyBySell, 'E', -1, 64) + `",
				"price": "` + strconv.FormatFloat(price, 'E', -1, 64) + `"
				}`
	resource := "/" + elasticClient.esDiffSizeIndex + "/orders"
	//GetLoggerInstance().Error("In elastic-client/IndexDiffSize - size: %f, price: %f ", size, price)
	resp := elasticClient.operation("POST", requestBody, resource)

	if resp == "" {
		GetLoggerInstance().Error("In elastic-client/IndexDiffSize. Response NIL")
		os.Exit(1)
	}
}

func (elasticClient *ElasticClient) IndexSubSize(t time.Time, productId string, sizeSell float64, sizeBuy float64, price float64) {
	sizeSellByBuy := sizeSell - sizeBuy
	requestBody := `{
				"time": "` + t.Format(time.RFC3339) + `",
				"product_id": "` + productId + `",
				"sub_size_sell_by_buy": "` + strconv.FormatFloat(sizeSellByBuy, 'E', -1, 64) + `",
				"price": "` + strconv.FormatFloat(price, 'E', -1, 64) + `"
				}`
	resource := "/" + elasticClient.esSubSizeIndex + "/orders"
	//GetLoggerInstance().Error("In elastic-client/IndexDiffSize - size: %f, price: %f ", size, price)
	resp := elasticClient.operation("POST", requestBody, resource)

	if resp == "" {
		GetLoggerInstance().Error("In elastic-client/IndexSubSize. Response NIL")
		os.Exit(1)
	}
}
