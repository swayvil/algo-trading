package nibiru

import (
	api "github.com/preichenberger/go-coinbase-exchange"
	//"time"
	//"fmt"
)

type OrdersStore struct {
	elasticClient *ElasticClient
}

func NewOrdersStore() *OrdersStore {
	elasticClient := NewElasticClient()
	return &OrdersStore{elasticClient}
}

func (store *OrdersStore) NewOrder(msg *api.Message) {
	switch msg.Type {
	case "match":
		{
			//GetLoggerInstance().ordersBooks.Println("[INFO] " + time.Now().Format("15:04:05") + " - OrdersStore - Adding match order")
			//GetLoggerInstance().Info("OrdersStore - Adding match order")
			store.elasticClient.IndexOrder(msg.Time.Time(), msg.ProductId, msg.Size, msg.Price, "")
			store.elasticClient.IndexOrder(msg.Time.Time(), msg.ProductId, msg.Size, msg.Price, msg.Side)
		}
	default:
		{
			// ignore other types
		}
	}
}