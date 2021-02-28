package main

import (
	"encoding/json"
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

type Config struct {
	PhilipsHueSensorUrl string
}

var config Config

func main() {

	reset := flag.Bool("r", false, "Reset the Stream Deck")
	noHardware := flag.Bool("n", false, "Run without Stream Deck and GPIO support")
	configPath := flag.String("c", "config.json", "Path to configuration file")
	flag.Parse()

	initConfig(*configPath)

	if *reset {
		fmt.Println("Resetting Stream Deck")
		resetStreamdeck()
	} else {
		var sd *streamdeck.StreamDeck

		if !*noHardware {
			initGpio()
			sd = initStreamdeck()
		}

		initMotionSensor(func(presence bool, lastPresence bool) {
			println("Presence:", presence)

			if sd != nil {
				if !presence {
					clearStreamDeckButtons(sd)
				} else if !lastPresence {
					initStreamDeckButtons(sd)
				}
			}
		})
		httpServer()
	}
}

func initConfig(configPath string) {

	file, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	config = Config{}
	err = decoder.Decode(&config)
	if err != nil {
		panic(err)
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

	clearStreamDeckButtons(sd)

	os.Exit(0)
}

func clearStreamDeckButtons(sd *streamdeck.StreamDeck) {
	blackButton := buttons.NewColourButton(color.Black)
	for i := 0; i < 6; i++ {
		sd.AddButton(i, blackButton)
	}
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

	buttonState := ButtonState{buttonIndex, images, 0}

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
	images      []string
	imageIndex  int
}

func initStreamdeck() *streamdeck.StreamDeck {
	sd, err := streamdeck.New()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found device [%s]\n", sd.GetName())

	initStreamDeckButtons(sd)

	go handleSignals(sd)

	return sd
}

func initStreamDeckButtons(sd *streamdeck.StreamDeck) {
	initImageToggleButton(sd, Dp2ButtonIndex, []string{"monitor-apple.jpg", "monitor-linux.jpg"}, func() { go blinkGpioPin(Dp2PinNumber) })
	initImageToggleButton(sd, Dp1ButtonIndex, []string{"monitor-apple.jpg", "monitor-linux.jpg"}, func() { go blinkGpioPin(Dp1PinNumber) })
	initImageToggleButton(sd, UsbButtonIndex, []string{"keyboard-apple.jpg", "keyboard-linux.jpg"}, func() { go blinkGpioPin(UsbPinNumber) })
	initImageToggleButton(sd, AllButtonIndex, []string{"all.jpg"}, func() {
		sd.GetButtonIndex(Dp1ButtonIndex).Pressed()
		sd.GetButtonIndex(Dp2ButtonIndex).Pressed()
		sd.GetButtonIndex(UsbButtonIndex).Pressed()
	})

	setSyncButton(sd, "sync-blue-on-black.jpg", "sync-blue-on-red.jpg")
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

func initMotionSensor(function func(bool, bool)) {

	if config.PhilipsHueSensorUrl != "" {
		go pollMotionSensor(function)
	}
}

type PhilipHueState struct {
	Presence    bool
	Lastupdated string
}

type PhilipHueResponse struct {
	State PhilipHueState
}

func pollMotionSensor(function func(bool, bool)) {

	var lastPresence = false
	for {
		println("Polling Philips Hue from", config.PhilipsHueSensorUrl)
		response, err := http.Get(config.PhilipsHueSensorUrl)
		if err != nil {
			fmt.Println(err)
		} else if response.StatusCode != 200 {
			fmt.Println("Unexpected response:", response.StatusCode, response.Status)
		} else {
			hueResponse := new(PhilipHueResponse)
			json.NewDecoder(response.Body).Decode(hueResponse)

			function(hueResponse.State.Presence, lastPresence)
			lastPresence = hueResponse.State.Presence
		}

		time.Sleep(10 * time.Second)
	}
}
