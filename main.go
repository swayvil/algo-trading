package main

import (
	nibiru "algo-trading/nibiru"
	"fmt"
	"time"
)

func main() {
	algo := nibiru.NewAlgo()
	algo.Run() // Start a ticker, which run in a goroutine
	
	wsocketClient := nibiru.NewWSocketClient()
	wsocketClient.Listen(nibiru.GetConfigInstance().Init.Crypto + "-" + nibiru.GetConfigInstance().Init.Currency)
	fmt.Printf("[INFO] %s - ALGO FINISHED\n", time.Now().Format("15:04:05"))
}