package nibiru

import (
	"encoding/json"
	"fmt"
	api "github.com/preichenberger/go-coinbase-exchange"
	"os"
	"time"
)

const simulationActivated bool = true

type GdaxClient struct {
	client          api.Client //Gdax API
	side            string
	productId       string
	cashAvailable   float64 // Updated in refreshCashCryptoAvailable()
	cryptoAvailable float64 // Updated in refreshCashCryptoAvailable()
	elasticClient   *ElasticClient
	simu            *Simulation
}

type Simulation struct { // Used for testing only
	cashAvailableSaved   float64
	cryptoAvailableSaved float64
	buyPrice             float64
	size                 float64 // for simulation a gdaxClient is always fully filled
	sellPrice            float64
	lastFillPrice        float64
	lastFillSize         float64
	fee                  float64
}

func NewGdaxClient() *GdaxClient {
	client := initClient()
	simu := Simulation{0, 0, 0, 0, 0, 0, 0, 0.3}
	elasticClient := NewElasticClient()
	t := &GdaxClient{client, GetConfigInstance().Init.Side, GetConfigInstance().Init.Crypto + "-" + GetConfigInstance().Init.Currency, 0, 0, elasticClient, &simu}
	t.initGdaxClient()
	return t
}

func initClient() api.Client {
	return api.Client{
		BaseURL:    GetConfigInstance().BaseURL,
		Secret:     GetConfigInstance().Account.Secret,
		Key:        GetConfigInstance().Account.Key,
		Passphrase: GetConfigInstance().Account.Passphrase,
	}
}

func (t *GdaxClient) initGdaxClient() {
	if GetConfigInstance().Init.Side != "buy" && GetConfigInstance().Init.Side != "sell" {
		GetLoggerInstance().Error("In gdaxClient/initGdaxClient. Incorrect value of side: %s. Values accepted: buy, sell", GetConfigInstance().Init.Side)
		os.Exit(1)
	}
	if GetConfigInstance().Init.Crypto != "BTC" && GetConfigInstance().Init.Crypto != "ETH" {
		GetLoggerInstance().Error("In gdaxClient/initGdaxClient. Incorrect value of crypto: %s. Values accepted: BTC, ETH", GetConfigInstance().Init.Crypto)
		os.Exit(1)
	}
	if GetConfigInstance().Init.Currency != "EUR" && GetConfigInstance().Init.Currency != "USD" {
		GetLoggerInstance().Error("In gdaxClient/initGdaxClient. Incorrect value of currency: %s. Values accepted: EUR, USD", GetConfigInstance().Init.Currency)
		os.Exit(1)
	}

	t.refreshCashCryptoAvailable() // Initialize cryptoAvailable and cashAvailable
	if GetConfigInstance().Init.Side == "buy" && t.cashAvailable <= 0 {
		GetLoggerInstance().Error("In gdaxClient/initGdaxClient. Side is buy but there is no cash available on the account")
		os.Exit(1)
	}
	if GetConfigInstance().Init.Side == "sell" && t.cryptoAvailable <= 0 {
		GetLoggerInstance().Error("In gdaxClient/initGdaxClient. Side is sell but there is no crypto currency available on the account")
		os.Exit(1)
	}
}

func (t *GdaxClient) getLastFill(side string) (lastPrice float64, lastSize float64, lastFee float64) {
	if simulationActivated {
		return t.getLastFill_simulation(side)
	}

	cursor := t.client.ListFills()
	var fills []api.Fill
	lastPrice = 0
	lastSize = 0
	lastFee = 0
	lastestTime := time.Time{}

	params := api.ListFillsParams{
		ProductId: t.productId,
	}
	cursor = t.client.ListFills(params)
	for cursor.HasMore {
		if err := cursor.NextPage(&fills); err != nil {
			GetLoggerInstance().Error("In gdaxClient/getLastFill. %s\n", err.Error(), time.Now().Format("15:04:05"))
			os.Exit(1)
		}

		for _, f := range fills {
			if f.Settled == true && f.Side == side && f.CreatedAt.Time().After(lastestTime) {
				lastestTime = f.CreatedAt.Time()
				lastPrice = f.Price
				lastSize = f.Size
				lastFee = f.Fee
			}
		}
	}
	return lastPrice, lastSize, lastFee
}

func (t *GdaxClient) getLastFill_simulation(side string) (lastPrice float64, lastSize float64, lastFee float64) {
	//GetLoggerInstance().Info("getLastFill_simulation. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
	return t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee
}

// Check if there is a capital gains if we sell
func (t *GdaxClient) canSell(sellPrice float64, sellSize float64) bool {
	lastPrice, lastSize, lastFee := t.getLastFill("buy")
	//GetLoggerInstance().Info("Gains estimation: %f", sellPrice*sellSize*(1-lastFee/100)-lastPrice*lastSize*(1-lastFee/100))
	//GetLoggerInstance().Info("canSell. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
	return lastPrice == 0 || sellPrice*sellSize*(1-lastFee/100)-lastPrice*lastSize*(1-lastFee/100) > GetConfigInstance().Init.MinGains
}

func (t *GdaxClient) refreshCashCryptoAvailable() { // TODO Get crypto from order not filled + stock
	if simulationActivated {
		t.refreshCashCryptoAvailable_simulation()
		return
	}

	accounts, err := t.client.GetAccounts()
	if err != nil {
		GetLoggerInstance().Error("In gdaxClient/checkStatus %s", err.Error())
		os.Exit(2)
	}

	for _, a := range accounts {
		if a.Currency == GetConfigInstance().Init.Crypto {
			t.cryptoAvailable = a.Available
		} else {
			if a.Currency == GetConfigInstance().Init.Currency {
				if a.Available > GetConfigInstance().Init.LimitCashAvailable {
					t.cashAvailable = GetConfigInstance().Init.LimitCashAvailable
				} else {
					t.cashAvailable = a.Available
				}
			}
		}
	}
	//GetLoggerInstance().Info("CashAvailable: %f, cryptoAvailable: %f", t.cashAvailable, t.cryptoAvailable)
}

func (t *GdaxClient) refreshCashCryptoAvailable_simulation() {
	t.cashAvailable = 8000
	t.cryptoAvailable = 0
	//GetLoggerInstance().Info("refreshCashCryptoAvailable_simulation. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
}

func (t *GdaxClient) checkStatus() string {
	if simulationActivated {
		return t.checkStatus_simulation()
	}

	t.refreshCashCryptoAvailable()
	if t.cryptoAvailable > 0 { // TODO CHANGE IT
		//GetLoggerInstance().Info("checkStatus: sell")
		t.side = "sell"
	} else {
		//GetLoggerInstance().Info("checkStatus: buy")
		t.side = "buy"
	}
	return t.side
}

func (t *GdaxClient) checkStatus_simulation() string {
	//GetLoggerInstance().Info("checkStatus_simulation: %s", t.side)
	return t.side
}

// DELETE /orders/<order-id>
//func (t *GdaxClient) cancelOrder() {
//	if simulationActivated {
//		t.cancelOrder_simulation()
//		return
//	}
//
//	var orders []api.Order
//
//	cursor := t.client.ListOrders()
//	for cursor.HasMore {
//		if err := cursor.NextPage(&orders); err != nil {
//			GetLoggerInstance().Error("In gdaxClient/cancelOrder %s", err.Error())
//			os.Exit(2)
//		}
//
//		for _, o := range orders {
//			if o.ProductId == t.productId {
//				if err := t.client.CancelOrder(o.Id); err != nil {
//					GetLoggerInstance().Error("In gdaxClient/cancelOrder, while canceling order %s", err.Error())
//					os.Exit(2)
//				}
//			}
//		}
//	}
//}

//func (t *GdaxClient) cancelOrder_simulation() {
//	//GetLoggerInstance().Info("cancelOrder_simulation 1. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
//	if t.simu.cashAvailableSaved != 0 { // If there is an order to cancel
//		t.cryptoAvailable = t.simu.cryptoAvailableSaved
//		t.cashAvailable = t.simu.cashAvailableSaved
//	}
//	t.simu.cryptoAvailableSaved = 0
//	t.simu.cashAvailableSaved = 0
//	t.simu.size = 0
//	//GetLoggerInstance().Info("cancelOrder_simulation 2. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
//}

// POST /orders
// Limit order
func (t *GdaxClient) createOrder(price float64, size float64) {
	if simulationActivated {
		t.createOrder_simulation(price, size)
		return
	}

	// Uncomment these lines in production
	//	order := api.Order{
	//		Type: "market",
	//		//Price:     price,
	//		Size:      size,
	//		Side:      t.side,
	//		ProductId: t.productId,
	//	}

	// savedOrder, err := t.client.CreateOrder(&order)
	//	t.elasticClient.IndexFillOrder(time.Now(), t.productId, size, price, t.side)
	//	if err != nil {
	//		GetLoggerInstance().Error("In gdaxClient/newLimitOrder, while creating new order %s", err.Error())
	//		os.Exit(2)
	//	}
	//	if savedOrder.Id == "" {
	//		GetLoggerInstance().Error("In gdaxClient/newLimitOrder, while creating new order, id is NULL")
	//		os.Exit(2)
	//	}
}

func (t *GdaxClient) createOrder_simulation(price float64, size float64) {
	//GetLoggerInstance().Info("createOrder_simulation 1. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
	t.simu.cryptoAvailableSaved = t.cryptoAvailable
	t.simu.cashAvailableSaved = t.cashAvailable
	if t.side == "buy" {
		t.cashAvailable -= price * size
		t.simu.buyPrice = price
		t.simu.size = size
	} else {
		t.cryptoAvailable -= size
		t.simu.sellPrice = price
		t.simu.size = size
	}
	//GetLoggerInstance().Info("createOrder_simulation 2. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
}

func (t *GdaxClient) fillGdaxClient_simulation(currentPrice float64) {
	if !simulationActivated {
		return
	}
	if t.simu.size == 0 { // there is no orders to execute
		GetLoggerInstance().Info("FillGdaxClient_simulation t.simu.size == 0")
		return
	}

	t.elasticClient.IndexFillOrder(time.Now(), t.productId, t.simu.size, currentPrice, t.side)
	switch t.side {
	case "buy":
		{
			GetLoggerInstance().Info("===> BUY %f crypto at %f", t.simu.size, currentPrice)
			t.cryptoAvailable += t.simu.size
			// t.cashAvailable was already decreased in createOrder
			t.simu.lastFillPrice = currentPrice
			t.simu.lastFillSize = t.simu.size
			//t.simu.buyPrice = 0
			t.simu.size = 0
			t.simu.sellPrice = 0
			t.simu.cashAvailableSaved = 0
			t.simu.cryptoAvailableSaved = 0
			t.side = "sell"
			//GetLoggerInstance().Info("FillGdaxClient_simulation BUY. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
		}
	case "sell":
		{
			//GetLoggerInstance().Info("FillGdaxClient_simulation SELL. currentPrice: %s, sellPrice: %f, buyPrice: %f, size: %f\n", currentPrice, t.simu.sellPrice, t.simu.buyPrice, t.simu.size)
			GetLoggerInstance().Info("<=== SELL %f crypto at %f. Gains: %f", t.simu.size, currentPrice, (t.simu.sellPrice-t.simu.buyPrice)*t.simu.size)
			// t.cryptoAvailable was already decreased in createOrder
			t.cashAvailable += t.simu.sellPrice * t.simu.size
			t.simu.lastFillPrice = currentPrice
			t.simu.lastFillSize = t.simu.size
			t.simu.buyPrice = 0
			t.simu.size = 0
			//t.simu.sellPrice = 0
			t.simu.cashAvailableSaved = 0
			t.simu.cryptoAvailableSaved = 0
			t.side = "buy"
			//GetLoggerInstance().Info("FillGdaxClient_simulation SELL. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
		}
	}
}

// GET /products/<product-id>/ticker
func (t *GdaxClient) GetTicker() (ask float64, bid float64) {
	//	if simulationActivated {
	//		return t.getTicker_simulation()
	//	}

	ticker, err := t.client.GetTicker(t.productId)
	if err != nil {
		GetLoggerInstance().Error("In gdaxClient/GetTicker: %s", err.Error())
		os.Exit(2)
	}

	//GetLoggerInstance().Info("Refresh ticker - ask: %f, bid: %f", ticker.Ask, ticker.Bid)
	return ticker.Ask, ticker.Bid
}

func (t *GdaxClient) getTicker_simulation() (ask float64, bid float64) {
	return 2000, 2000
}

func (t *GdaxClient) UpdatePosition(side string, price float64) {
	GetLoggerInstance().Info("UpdatePosition: %s", side)
	//GetLoggerInstance().Info("UpdatePosition 1. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
	switch side {
	case "buy":
		{
			// check status of current order if still opened, else switch of side
			if t.checkStatus() != "buy" {
				return
			}
			// cancel open order
			//GetLoggerInstance().Info("UpdatePosition: cancel buy order")
			//t.cancelOrder()
			// new ask order on price
			size := (t.cashAvailable - GetConfigInstance().Init.CashReserve) / price
			if size <= 0 {
				GetLoggerInstance().Error("UpdatePosition: Not enough cash %f", t.cashAvailable)
				os.Exit(3)
			}
			GetLoggerInstance().Info("=> UpdatePosition: create buy order price: %f, size: %f", price, size)
			t.createOrder(price, size)
			t.fillGdaxClient_simulation(price)
		}
	case "sell":
		{
			// check status of current order if still opened, else switch of side
			if t.checkStatus() != "sell" {
				return
			}
			cryptoAvailableTmp := t.cryptoAvailable // TODO Use refreshCashCryptoAvailable()
			if cryptoAvailableTmp == 0 {
				cryptoAvailableTmp = t.simu.cryptoAvailableSaved // t.cryptoAvailable is null after a createOrder
			}
			if !t.canSell(price, cryptoAvailableTmp) {
				GetLoggerInstance().Info("No gain, ignore sell")
				return // we won't make a gain, so we don't sell
			}
			// cancel open order
			//GetLoggerInstance().Info("UpdatePosition: cancel sell order")
			//t.cancelOrder()
			// new bid order on price
			GetLoggerInstance().Info("=> UpdatePosition: create sell order price: %f, size: %f", price, t.cryptoAvailable)
			t.createOrder(price, t.cryptoAvailable)
			t.fillGdaxClient_simulation(price)
		}
	default:
		{
			GetLoggerInstance().Error("In gdaxClient/UpdatePosition, incorrect side ", time.Now().Format("15:04:05"), side)
			os.Exit(2)
		}
	}
	//GetLoggerInstance().Info("UpdatePosition 2. side: %s, cashAvailable: %f, cryptoAvailable: %f, cashAvailableSaved: %f, cryptoAvailableSaved: %f, buyPrice: %f, size: %f, sellPrice: %f, lastFillPrice: %f, lastFillSize: %f, fee: %f", t.side, t.cashAvailable, t.cryptoAvailable, t.simu.cashAvailableSaved, t.simu.cryptoAvailableSaved, t.simu.buyPrice, t.simu.size, t.simu.sellPrice, t.simu.lastFillPrice, t.simu.lastFillSize, t.simu.fee)
}

// GET /orders/<order-id>
func (t *GdaxClient) PrintOrders() {
	cursor := t.client.ListOrders()
	var orders []api.Order

	cursor = t.client.ListOrders(api.ListOrdersParams{Status: "done"})
	for cursor.HasMore {
		if err := cursor.NextPage(&orders); err != nil {
			GetLoggerInstance().Error("In PrintOrders: %s", err.Error())
		}

		for _, o := range orders {
			js, _ := json.Marshal(o)
			fmt.Println(string(js))
		}
	}
}
