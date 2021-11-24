package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	adapter = bluetooth.DefaultAdapter

	heartRateServiceUUID        = bluetooth.ServiceUUIDHeartRate
	heartRateCharacteristicUUID = bluetooth.CharacteristicUUIDHeartRateMeasurement
)

func connectAddress() string {
	if len(os.Args) < 2 {
		println("usage: heartrate-monitor [address]")
		os.Exit(1)
	}

	// look for device with specific name
	address := os.Args[1]

	return address
}

var overdueCounter = 0
var receivedTime = time.Now().Unix()

func init() {
	ioutil.WriteFile("reconnect.log", []byte(" \n"), 0744)
	ioutil.WriteFile("bluetooth.log", []byte(strconv.FormatInt(receivedTime, 10)+" - start to testing...\n"), 0744)
}

func run() {
	f, err := os.OpenFile("bluetooth.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	r, _ := os.OpenFile("reconnect.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	defer r.Close()
	defer func() {
		run()
	}()
	receivedTime := time.Now().Unix()
	println("enabling")
	// ioutil.WriteFile("bluetooth.log", []byte(strconv.FormatInt(receivedTime, 10)+" - start to testing...\n"), 0744)

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	ch := make(chan bluetooth.ScanResult, 1)

	// Start scanning.
	println("scanning...")
	err = adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		println("found device:", result.Address.String(), result.RSSI, result.LocalName())
		if result.Address.String() == connectAddress() {
			adapter.StopScan()
			ch <- result
		}
	})

	var device *bluetooth.Device
	select {
	case result := <-ch:
		device, err = adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			println(err.Error())
			return
		}

		println("connected to ", result.Address.String())
	}

	// get services
	println("discovering services/characteristics")
	srvcs, err := device.DiscoverServices([]bluetooth.UUID{heartRateServiceUUID})
	must("discover services", err)

	if len(srvcs) == 0 {
		panic("could not find heart rate service")
	}

	srvc := srvcs[0]

	println("found service", srvc.UUID().String())

	chars, err := srvc.DiscoverCharacteristics([]bluetooth.UUID{heartRateCharacteristicUUID})
	if err != nil {
		println(err)
	}

	if len(chars) == 0 {
		panic("could not find heart rate characteristic")
	}

	char := chars[0]
	println("found characteristic", char.UUID().String())

	char.EnableNotifications(func(buf []byte) {
		receivedTime = time.Now().Unix()
		ioutil.WriteFile("receivedTime.log", []byte(strconv.FormatInt(receivedTime, 10)+"\n"), 0744)
		f.WriteString(strconv.FormatInt(receivedTime, 10) + " - received\n")
		// println("data:", uint8(buf[1]))
		fmt.Println("receivedTime", receivedTime)
	})

	checkTimer := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-checkTimer.C:
			// ioutil.WriteFile("bluetooth.log", []byte("check...\n"), 0744)
			checkTime := time.Now().Unix()
			fmt.Println("checkTime", checkTime)
			if checkTime-receivedTime > 3 {
				overdueCounter++
				f.WriteString(strconv.FormatInt(checkTime, 10) + " - overdue\n")
				if overdueCounter > 2 {
					r.WriteString(strconv.FormatInt(checkTime, 10) + " - reconnect\n")
					device.Disconnect()
					run()
				}
			} else {
				overdueCounter = 0
				f.WriteString(strconv.FormatInt(checkTime, 10) + " - pass\n")
			}
		}
	}
}

func main() {
	run()
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
