package nibiru

import (
	ws "github.com/gorilla/websocket"
	api "github.com/preichenberger/go-coinbase-exchange"
	"os"
	"time"
)

const (
	// Time allowed to write a message to the peer
	writeWait = time.Duration(10) * time.Second

	// Time allowed to read the next pong message from the peer
	// If a socket doesn’t answer within this time range, consider Client is disconnected
	pongWait = time.Duration(30) * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10 // Send pings period

	//connIsLostLimit = time.Duration(8) * time.Second // If no msg received after this period, connection is considered as lost
)

type WSocketClient struct {
	wsConn      *ws.Conn
	ordersStore *OrdersStore
	//lastMsgReadTS time.Time
	listen     bool
	pingTicker *time.Ticker
}

type WsHeartbeatMessage struct {
	Type string `json:"type"`
	On   bool   `json:"on"`
}

type WsSubscribeMessage struct {
	Type       string   `json:"type"`
	ProductIds []string `json:"product_ids"`
}

func NewWSocketClient() *WSocketClient {
	return &WSocketClient{getConnection(), NewOrdersStore() /*time.Now(), */, true, nil}
}

func getConnection() *ws.Conn {
	var wsDialer ws.Dialer
	GetLoggerInstance().Info("getConnection")
	wsConn, _, err := wsDialer.Dial(GetConfigInstance().WssURL, nil)
	if err != nil {
		GetLoggerInstance().Error("In wsocket-client/getConnection: %s", err.Error())
		os.Exit(2)
	}
	return wsConn
}

func (l *WSocketClient) Listen(productid string) {
	GetLoggerInstance().Info("Listen %s, on: %s", productid, GetConfigInstance().WssURL)

	message := api.Message{}
	//go wsclient.checkConnection()

	defer l.wsConn.Close()
	for true {
		l.heartbeat(true)
		l.ping()

		subscribe := WsSubscribeMessage{Type: "subscribe", ProductIds: []string{productid}}
		if err := l.wsConn.WriteJSON(subscribe); err != nil {
			GetLoggerInstance().Error("In wsocket-client/Listen, during Subscribe: %s", err.Error())
			os.Exit(2)
		}

		l.listen = true
		GetLoggerInstance().Info("Listening")
		for l.listen {
			err := l.wsConn.ReadJSON(&message)
			if err != nil {
				GetLoggerInstance().Error("In wsocket-client/Listen: %s", err.Error())
				l.listen = false
				break
			} else {
				//GetLoggerInstance().Info("OrderId: %s", message.Type)
				l.ordersStore.NewOrder(&message)
				//l.lastMsgReadTS = time.Now()
			}
		}
		GetLoggerInstance().Info("Close connection")
		l.pingTicker.Stop()
		// If here, it is because the connection is on error, so the connection might be already closed
		//l.wsConn.Close()
		// Restart the connection
		l.wsConn = getConnection()
		GetLoggerInstance().Info("Restart listen")
	}
}

// Turns heartbeat on/off on the Connection
// message received have type=heartbeat
func (l *WSocketClient) heartbeat(on bool) {
	msg := WsHeartbeatMessage{Type: "heartbeat", On: on}

	if err := l.wsConn.WriteJSON(msg); err != nil {
		GetLoggerInstance().Error("In wsocket-client/heartbeat: %s", err.Error())
		os.Exit(2)
	}
	GetLoggerInstance().Info("Websocket heartbeat activated")
}

func (l *WSocketClient) ping() {
	l.pingTicker = time.NewTicker(pingPeriod) // Send pings on a regular interval

	// Set read deadline to a time less than next expected pong
	l.wsConn.SetReadDeadline(time.Now().Add(pongWait))
	// Reset the read deadline when a pong is received
	l.wsConn.SetPongHandler(func(string) error {
		//GetLoggerInstance().Info("PONG")
		l.wsConn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		for {
			select {
			case <-l.pingTicker.C:
				l.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
				//GetLoggerInstance().Info("PING")
				// If a pong goes missing, the read methods will return with the read past deadline error
				if err := l.wsConn.WriteMessage(ws.PingMessage, []byte{}); err != nil {
					GetLoggerInstance().Error("In wsocket-client/ping, connection seems lost")
					// Connection seems lost
					l.listen = false
					return
				}
			}
		}
	}()
}

//func (l *WSocketClient) checkConnection() {
//	ticker := time.NewTicker(connIsLostLimit)
//	defer ticker.Stop()
//	for {
//		select {
//		case <-ticker.C:
//			//GetLoggerInstance().Info("checkConnection: %f", l.lastMsgReadTS.Add(connIsLostLimit).Sub(time.Now()).Seconds())
//			if l.lastMsgReadTS.Add(connIsLostLimit).Sub(time.Now()) < time.Duration(0) { // No msg received since connIsLostLimit seconds
//				GetLoggerInstance().Error("checkConnection, connection seems lost: %f", l.lastMsgReadTS.Add(connIsLostLimit).Sub(time.Now()).Seconds())
//				// Connection seems lost
//				l.listen = false
//			}
//		}
//	}
//}
