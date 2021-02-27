package main

import (
	"fmt"
	"github.com/magicmonkey/go-streamdeck/actionhandlers"
	"log"
	"net/http"
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
	initStreamdeck()
	initGpio()

	httpServer()
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

func initStreamdeck() {
	sd, err := streamdeck.New()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found device [%s]\n", sd.GetName())

	dp1Button, err := buttons.NewImageFileButton(imagePath("monitor.jpg"))
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		dp1Button.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { blinkGpioPin(Dp1PinNumber)}))
		sd.AddButton(3, dp1Button)
	}

	dp2Button, err := buttons.NewImageFileButton(imagePath("monitor.jpg"))
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		dp2Button.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { blinkGpioPin(Dp2PinNumber)}))
		sd.AddButton(0, dp2Button)
	}

	kbButton, err := buttons.NewImageFileButton(imagePath("keyboard.jpg"))
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		kbButton.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { blinkGpioPin(UsbPinNumber)}))
		sd.AddButton(1, kbButton)
	}

	allButton, err := buttons.NewImageFileButton(imagePath("all.jpg"))
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		allButton.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) {
			blinkGpioPin(Dp1PinNumber)
			blinkGpioPin(Dp2PinNumber)
			blinkGpioPin(UsbPinNumber)
		}))
		sd.AddButton(4, allButton)
	}
}
