package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"time"

	"log"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/niwla23/smard-go"
)

type convertedData struct {
	values     map[string]int
	timestamps map[string]time.Time
}

func getTimeFrameWidthFromData[T any](data []T) time.Duration {
	row1 := reflect.ValueOf(data[len(data)-2]).FieldByName("Timestamp").Interface().(time.Time)
	row2 := reflect.ValueOf(data[len(data)-1]).FieldByName("Timestamp").Interface().(time.Time)

	return row2.Sub(row1)
}

func WatthoursToWatt(watthours float64, timeframe time.Duration) float64 {
	return watthours / timeframe.Hours()
}

// this takes the arrays of structs from smard-go, finds the latest value for each column and returns them as a map
func convertDataToLatestRowMap[T any](data []T) convertedData {
	timeframeWidth := getTimeFrameWidthFromData(data)
	latestValues := make(map[string]int)
	latestTimestamps := make(map[string]time.Time)

	for _, row := range data {
		v := reflect.ValueOf(row)
		typeOfS := v.Type()

		rowTimestamp := v.FieldByName("Timestamp").Interface().(time.Time)
		for i := 0; i < v.NumField(); i++ {
			name := typeOfS.Field(i).Name

			if name == "Timestamp" {
				continue
			}

			val := v.Field(i).Interface().(int)

			if val != -1 {
				latestValues[name] = int(WatthoursToWatt(float64(val), timeframeWidth))
				fmt.Printf("Wh: %v; W: %v\n", val, latestValues[name])
				latestTimestamps[name] = rowTimestamp
			}
		}
	}
	return convertedData{latestValues, latestTimestamps}
}

func PublishDataForCategory[T any](client mqtt.Client, categoryName string, getData func(time.Time, time.Time) ([]T, error)) {
	log.Printf("getting data for category %s", categoryName)

	now := time.Now()
	start := now.Add(time.Duration(-6) * time.Hour)
	data, err := getData(start, now)

	if err != nil {
		panic(err)
	}

	latestData := convertDataToLatestRowMap(data)
	log.Printf("publishing data for category %s", categoryName)

	for key, value := range latestData.values {
		client.Publish(fmt.Sprintf("smard/%s/%s", categoryName, key), 1, true, fmt.Sprint(value))
		client.Publish(fmt.Sprintf("smard/%s/%s/timestamp", categoryName, key), 1, true, fmt.Sprint(latestData.timestamps[key].UnixMilli()))
	}
}

func randToken(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func main() {

	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker(os.Getenv("MQTT_SERVER")).SetClientID(fmt.Sprintf("smard-%s", randToken(6)))
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	for {
		PublishDataForCategory(c, "production", smard.GetProductionData)
		PublishDataForCategory(c, "production_forecast", smard.GetProductionForecastData)
		PublishDataForCategory(c, "consumption", smard.GetConsumptionData)

		time.Sleep(5 * time.Minute)
	}
}
