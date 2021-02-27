package main

import (
	"flag"
	"fmt"
	"github.com/magicmonkey/go-streamdeck/actionhandlers"
	"image/color"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	streamdeck "github.com/magicmonkey/go-streamdeck"
	"github.com/magicmonkey/go-streamdeck/buttons"
	_ "github.com/magicmonkey/go-streamdeck/devices"
	"github.com/stianeikeland/go-rpio"
)

const Dp1PinNumber = 22
const Dp2PinNumber = 23
const UsbPinNumber = 24
const ImageDir = "images"
const HttpListenAddr = ":8080"

func main() {

	reset := flag.Bool("reset", false, "Reset the Stream Deck")
	flag.Parse()

	if *reset {
		fmt.Print("Resetting Stream Deck")
		resetStreamdeck()
	} else {
		initGpio()
		initStreamdeck()
		httpServer()
	}
}

func resetStreamdeck() {
	sd, err := streamdeck.Open()
	if err != nil {
		panic(err)
	}
	sd.ResetComms()
}

func handleSignals(sd *streamdeck.StreamDeck) {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

    <-c

    blackButton := buttons.NewColourButton(color.Black)
    for i := 0; i < 6; i++ {
		sd.AddButton(i, blackButton)
	}

    os.Exit(0)
}

func httpServer() {
	fmt.Printf("Starting server at %s\n", HttpListenAddr)

	http.HandleFunc("/usb1", blinkPinFunction(UsbPinNumber))
	http.HandleFunc("/dp1", blinkPinFunction(Dp1PinNumber))
	http.HandleFunc("/dp2", blinkPinFunction(Dp2PinNumber))

    if err := http.ListenAndServe(HttpListenAddr, nil); err != nil {
        log.Fatal(err)
    }
}

func blinkPinFunction(pin int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		println("Blinking pin", pin)
		blinkGpioPin(pin)
		w.WriteHeader(201)
	}
}

func initGpio() {
	err := rpio.Open()
	if err != nil {
		panic(err)
	}
}

func blinkGpioPin(pinNumber int) {
	pin := rpio.Pin(pinNumber)

	println("Blinking", pin)

	pin.Output()
	pin.High()
	time.Sleep(time.Millisecond * 200)
	pin.Low()
}

func imagePath(image string) string {
	return fmt.Sprintf("%s/%s", ImageDir, image)
}

func initImageToggleButton(image string, function func()) *buttons.ImageFileButton {
	button, err := buttons.NewImageFileButton(imagePath(image))
	if err != nil {
		panic(err)
	}

	button.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { function() }))
	return button
}

func initStreamdeck() {
	sd, err := streamdeck.New()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found device [%s]\n", sd.GetName())

	sd.AddButton(0, initImageToggleButton("monitor.jpg", func() { blinkGpioPin(Dp2PinNumber)} ))
	sd.AddButton(1, initImageToggleButton("keyboard.jpg", func() { blinkGpioPin(UsbPinNumber)} ))
	sd.AddButton(3, initImageToggleButton("monitor.jpg", func() { blinkGpioPin(Dp1PinNumber)} ))
	sd.AddButton(4, initImageToggleButton("all.jpg", func() {
		blinkGpioPin(Dp1PinNumber)
		blinkGpioPin(Dp2PinNumber)
		blinkGpioPin(UsbPinNumber)
	} ))

	handleSignals(sd)
}
