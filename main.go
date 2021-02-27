package main

import (
	"fmt"
	"github.com/magicmonkey/go-streamdeck/actionhandlers"
	"net/http"
	"time"

	streamdeck "github.com/magicmonkey/go-streamdeck"
	"github.com/magicmonkey/go-streamdeck/buttons"
	_ "github.com/magicmonkey/go-streamdeck/devices"
)

func main() {
	initStreamdeck()

	time.Sleep(6000 * time.Second)
}

func initStreamdeck() {
	sd, err := streamdeck.New()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found device [%s]\n", sd.GetName())

	dp1Button, err := buttons.NewImageFileButton("monitor.jpg")
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		dp1Button.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { http.Get("http://192.168.18.28:8080/dp1") }))
		sd.AddButton(3, dp1Button)
	}

	dp2Button, err := buttons.NewImageFileButton("monitor.jpg")
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		dp2Button.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { http.Get("http://192.168.18.28:8080/dp2") }))
		sd.AddButton(0, dp2Button)
	}

	kbButton, err := buttons.NewImageFileButton("keyboard.jpg")
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		kbButton.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { http.Get("http://192.168.18.28:8080/usb1") }))
		sd.AddButton(1, kbButton)
	}

	allButton, err := buttons.NewImageFileButton("all.jpg")
	if err != nil {
		fmt.Printf("Failure [${err}]")
	} else {
		allButton.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) {
			http.Get("http://192.168.18.28:8080/dp1")
			http.Get("http://192.168.18.28:8080/dp2")
			http.Get("http://192.168.18.28:8080/usb1")
		}))
		sd.AddButton(4, allButton)
	}
}
