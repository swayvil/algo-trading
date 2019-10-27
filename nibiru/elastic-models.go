package nibiru

import (
	"encoding/json"
)

type ESResponse struct {
	Took         int              `json:"took"`
	TimeOut      bool             `json:"timed_out"`
	Shards       ShardsType       `json:"_shards"`
	Hits         HitsType         `json:"hits"`
	Aggregations AggregationsType `json:"aggregations"`
}

type ShardsType struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

type HitsType struct {
	Total    int             `json:"total"`
	MaxScore float64         `json:"max_score"`
	Hits     []HitsArrayType `json:"hits"`
}

type HitsArrayType struct {
	Index  string     `json:"_index"`
	Type   string     `json:"_type"`
	Id     string     `json:"_id"`
	Score  float64    `json:"_score"`
	Source SourceType `json:"_source"`
}

type SourceType struct {
	MatchTime string  `json:"matchTime"`
	ProductId string  `json:"product_id"`
	Size      float64 `json:"size,string"`
	Price     float64 `json:"price,string"`
	Side      string  `json:"side"`
}

type AggregationsType struct {
	PriceRanges PriceRangesType `json:"price_ranges"`
}

type PriceRangesType struct {
	Buckets []BucketsType `json:"buckets"`
}

type BucketsType struct {
	Key          string      `json:"key"`
	From         json.Number `json:"from,Number"`
	FromAsString string      `json:"from_as_string"`
	To           json.Number `json:"to,Number"`
	ToAsString   string      `json:"to_as_string"`
	DocCount     int         `json:"doc_count"`
	Result       ResultType  `json:"result"`
}

type ResultType struct {
	Value float64 `json:"value"`
}
