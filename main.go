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

const Dp2ButtonIndex = 0
const UsbButtonIndex = 1
const SyncButtonIndex = 2
const Dp1ButtonIndex = 3
const AllButtonIndex = 4

var syncState = false

func main() {

	reset := flag.Bool("r", false, "Reset the Stream Deck")
	noHardware := flag.Bool("n", false, "Run without Stream Deck and GPIO support")
	flag.Parse()

	if *reset {
		fmt.Println("Resetting Stream Deck")
		resetStreamdeck()
	} else {
		if !*noHardware {
			initGpio()
			initStreamdeck()
		}
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

func initImageToggleButton(sd *streamdeck.StreamDeck, buttonIndex int, images []string, function func()) {

	buttonState := ButtonState{ buttonIndex, images, 0 }

	setImageToggleButton(sd, buttonState, function)
}

func setImageToggleButton(sd *streamdeck.StreamDeck, buttonState ButtonState, function func()) {

	button, err := buttons.NewImageFileButton(imagePath(buttonState.images[buttonState.imageIndex]))
	if err != nil {
		panic(err)
	}

	button.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) {
		buttonState.imageIndex++
		if buttonState.imageIndex >= len(buttonState.images) {
			buttonState.imageIndex = 0
		}

		setImageToggleButton(sd, buttonState, function)
		if !syncState {
			function()
		}
	}))

	sd.AddButton(buttonState.buttonIndex, button)
}

type ButtonState struct {
	buttonIndex int
	images []string
	imageIndex int
}

func initStreamdeck() {
	sd, err := streamdeck.New()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found device [%s]\n", sd.GetName())

	initImageToggleButton(sd, Dp2ButtonIndex, []string{"monitor-apple.jpg", "monitor-linux.jpg"}, func() { blinkGpioPin(Dp2PinNumber)} )
	initImageToggleButton(sd, Dp1ButtonIndex, []string{"monitor-apple.jpg", "monitor-linux.jpg"}, func() { blinkGpioPin(Dp1PinNumber)} )
	initImageToggleButton(sd, UsbButtonIndex, []string{"keyboard-apple.jpg", "keyboard-linux.jpg"}, func() { blinkGpioPin(UsbPinNumber)} )
	initImageToggleButton(sd, AllButtonIndex, []string{"all.jpg"}, func() {
		blinkGpioPin(Dp1PinNumber)
		blinkGpioPin(Dp2PinNumber)
		blinkGpioPin(UsbPinNumber)
	})

	setSyncButton(sd, "sync-blue-on-black.jpg", "sync-blue-on-red.jpg")

	go handleSignals(sd)
}

func setSyncButton(sd *streamdeck.StreamDeck, noSyncImage string, syncImage string) {

	var image = noSyncImage
	if syncState {
		image = syncImage
	}

	syncButton, err := buttons.NewImageFileButton(imagePath(image))
	if err != nil {
		panic(err)
	}
	syncButton.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) {
		syncState = !syncState
		setSyncButton(sd, noSyncImage, syncImage)
	}))

	sd.AddButton(SyncButtonIndex, syncButton)
}
