package nibiru

import (
	"fmt"
	"time"
)

type Algo struct {
	periodLong     int //minutes
	periodShort    int //minutes
	thresholdShort float64
	thresholdLong  float64
	volumeTicker   *time.Ticker
	elasticClient  *ElasticClient
	gdaxClient     *GdaxClient
}

func NewAlgo() *Algo {
	elasticClient := NewElasticClient()
	return &Algo{GetConfigInstance().Algo.PeriodLong, GetConfigInstance().Algo.PeriodShort, GetConfigInstance().Algo.ThresholdShort,
		GetConfigInstance().Algo.ThresholdLong, nil, elasticClient, NewGdaxClient()}
}

func (algo *Algo) Run() {
	//GetLoggerInstance().Info("In elastic-client/Aggregate. TEST: %f", algo.elasticClient.Aggregate("size", GetConfigInstance().Algo.PeriodLong, "avg"))

	GetLoggerInstance().Info("Run Algo ticker")
	startAlgoTime := time.Now()
	algo.volumeTicker = time.NewTicker(time.Duration(algo.periodShort) * time.Minute)
	go func() {
		for t := range algo.volumeTicker.C {
			GetLoggerInstance().Info("Algo/Run")
			if time.Now().Sub(startAlgoTime) >= time.Duration(algo.periodLong)*time.Minute { // Wait for initialization period
				var side = "buy"
				var sideOpposite = "sell"
				portfolioSide := algo.gdaxClient.checkStatus()
				if portfolioSide == "buy" { // means we have cash and not crypto
					side = "sell" // If the side is sell this indicates the maker was a sell order and the match is considered an up-tick. A buy side match is a down-tick.
					// important sell side orders volume means the price is going up, that's what we want to detect when we want to buy
					sideOpposite = "buy"
				}
				GetLoggerInstance().Info("Algo/Run - Side: %s", side)
				// Average volume orders in the last periodShort minutes
				sumVolumeShort := algo.elasticClient.Aggregate("size", algo.periodShort, "sum", side)
				sumVolumeShortOpposite := algo.elasticClient.Aggregate("size", algo.periodShort, "sum", sideOpposite)

				// Sum volume orders in the last periodLong
				sumVolumeLong := algo.elasticClient.Aggregate("size", algo.periodLong, "sum", side)
				sumVolumeLongOpposite := algo.elasticClient.Aggregate("size", algo.periodLong, "sum", sideOpposite)

				GetLoggerInstance().Info("Algo/Run - volume short: %f", sumVolumeShort)
				GetLoggerInstance().Info("Algo/Run - sumVolumeLong: %f", sumVolumeLong)
				GetLoggerInstance().Info("Algo/Run - volume long rapporte sur short periode: %f", sumVolumeLong/float64(algo.periodLong/algo.periodShort))
				GetLoggerInstance().Info("Algo/Run - volume short Opposite: %f", sumVolumeShortOpposite)
				GetLoggerInstance().Info("Algo/Run - sumVolumeLong Opposite: %f", sumVolumeLongOpposite)
				GetLoggerInstance().Info("Algo/Run - volume long rapporte sur short periode Opposite: %f", sumVolumeLongOpposite/float64(algo.periodLong/algo.periodShort))

				price := algo.elasticClient.GetLatestPrice() // Only for testing, in prod we create market order
				
				if side == "sell" {
					algo.elasticClient.IndexDiffSize(t, GetConfigInstance().Init.Crypto + "-" + GetConfigInstance().Init.Currency, sumVolumeShort, sumVolumeShortOpposite, price)
					algo.elasticClient.IndexSubSize(t, GetConfigInstance().Init.Crypto + "-" + GetConfigInstance().Init.Currency, sumVolumeShort, sumVolumeShortOpposite, price)
				} else {
					algo.elasticClient.IndexDiffSize(t, GetConfigInstance().Init.Crypto + "-" + GetConfigInstance().Init.Currency, sumVolumeShortOpposite, sumVolumeShort, price)
					algo.elasticClient.IndexSubSize(t, GetConfigInstance().Init.Crypto + "-" + GetConfigInstance().Init.Currency, sumVolumeShortOpposite, sumVolumeShort, price)
				}
								
				// Volume side des periodShort dernieres minutes / Volume sideOpposite des periodShort dernieres minutes > thresholdShort
				if sumVolumeShort/sumVolumeShortOpposite > algo.thresholdShort {
					GetLoggerInstance().Info("Algo/Run - VALIDATE: %f", sumVolumeShort/sumVolumeShortOpposite)
					algo.gdaxClient.UpdatePosition(portfolioSide, price)
				} else {
					// Volume side des periodLong dernieres minutes / Volume sideOpposite des periodLong dernieres minutes > thresholdLong
					if sumVolumeLong/sumVolumeLongOpposite > algo.thresholdLong {
						GetLoggerInstance().Info("Algo/Run - VALIDATE: %f", sumVolumeLong/sumVolumeLongOpposite)
						algo.gdaxClient.UpdatePosition(portfolioSide, price)
					}
				}
			}
		}
	}()
}

func (algo *Algo) Stop() {
	algo.volumeTicker.Stop()
	fmt.Println("Ticker stopped")
}
