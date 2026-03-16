package main

import (
	"flag"
	"io"
	"log"
	"os"
	"time"

	"github.com/otfabric/go-serial"
)

var (
	address  string
	baudrate int
	databits int
	stopbits int
	parity   string

	message string
)

func main() {
	flag.StringVar(&address, "a", "/dev/ttyUSB0", "address")
	flag.IntVar(&baudrate, "b", 115200, "baud rate")
	flag.IntVar(&databits, "d", 8, "data bits")
	flag.IntVar(&stopbits, "s", 1, "stop bits")
	flag.StringVar(&parity, "p", "N", "parity (N/E/O)")
	flag.StringVar(&message, "m", "serial", "message")
	flag.Parse()

	parityVal, err := serial.ParseParity(parity)
	if err != nil {
		log.Fatal(err)
	}
	dataBitsVal, err := serial.ParseDataBits(databits)
	if err != nil {
		log.Fatal(err)
	}
	stopBitsVal, err := serial.ParseStopBits(stopbits)
	if err != nil {
		log.Fatal(err)
	}
	baudRateVal, err := serial.ParseBaudRate(baudrate)
	if err != nil {
		log.Fatal(err)
	}
	config := serial.DefaultConfig(address)
	config.BaudRate = baudRateVal
	config.DataBits = dataBitsVal
	config.StopBits = stopBitsVal
	config.Parity = parityVal
	config.Timeout = 30 * time.Second
	log.Printf("connecting %+v", config)
	port, err := serial.Open(&config)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("connected")
	defer func() {
		if err := port.Close(); err != nil {
			log.Printf("close: %v", err)
		}
	}()

	if _, err = port.Write([]byte(message)); err != nil {
		log.Fatal(err)
	}
	if _, err = io.Copy(os.Stdout, port); err != nil {
		log.Fatal(err)
	}
}
