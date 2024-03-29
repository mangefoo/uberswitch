package main

import (
	"bytes"
	"encoding/json"
    "flag"
    "fmt"
    "github.com/magicmonkey/go-streamdeck/actionhandlers"
    "image/color"
    "log"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    streamdeck "github.com/magicmonkey/go-streamdeck"
    "github.com/magicmonkey/go-streamdeck/buttons"
    _ "github.com/magicmonkey/go-streamdeck/devices"
    "github.com/stianeikeland/go-rpio"
)

const (
    ImageDir       = "images"
    HttpListenAddr = ":8080"
)

var syncState = false

type Switch struct {
    Name string
    Type string
    ButtonIndex int
    GPIOPin int
    Images []string
}

type Config struct {
    PhilipsHueSensorUrl string
    MotionSensorThresholdSecs int
    Switches []Switch
	SensorRelayUrl string
}

type SensorReport struct {
	Reporter string `json:"reporter"`
	Topic string `json:"topic"`
	Sensors map[string]string `json:"sensors"`
}

var config Config
var statePath *string
var noHardware *bool
var sd *streamdeck.StreamDeck
var toggleMutex sync.Mutex

func main() {

    reset := flag.Bool("r", false, "Reset the Stream Deck")
    noHardware = flag.Bool("n", false, "Run without Stream Deck and GPIO support")
    configPath := flag.String("c", "config.json", "Path to configuration file")
    statePath = flag.String("s", "state.json", "Path to state file")
    flag.Parse()

    initConfig(*configPath)
    RestoreButtonState(*statePath)

    if *reset {
        fmt.Println("Resetting Stream Deck")
        resetStreamdeck()
    } else {
        if !*noHardware {
            initGpio()
            initStreamdeck()
        }

        initMotionSensor(func(presence bool) {
            if sd != nil {
                if presence {
                    println("Turning display on")
                    initStreamDeckButtons()
                } else {
                    println("Turning display off")
                    clearStreamDeckButtons(func() { initStreamDeckButtons() })
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

func handleSignals() {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

    <-c

    clearStreamDeckButtons(func() {})

    os.Exit(0)
}

func clearStreamDeckButtons(pressFunction func()) {
    blackButton := buttons.NewColourButton(color.Black)
    blackButton.SetActionHandler(actionhandlers.NewCustomAction(func(streamdeck.Button) { pressFunction() }))
    for i := 0; i < 6; i++ {
        sd.AddButton(i, blackButton)
    }
}

func getSwitchFunction(sw Switch) func() {
	switch sw.Type {
    case "toggle":
        return func() {
            log.Print("Toggle pin ", sw.GPIOPin)
            if !syncState {
                blinkGpioPin(sw.GPIOPin)
            }
            toggleImageButton(sw)
        }
    case "toggleAll":
        toggleSwitches := filterSwitches(config.Switches, func(s Switch) bool {
            return s.Type == "toggle"
        })

        return func() {
            log.Println("Toggling all pins")
            if !syncState {
                for _, toggleSwitch := range toggleSwitches {
                    log.Print("Toggling pin ", toggleSwitch.GPIOPin)
                    blinkGpioPin(toggleSwitch.GPIOPin)
                    toggleImageButton(toggleSwitch)
                }
            }
            toggleImageButton(sw)
        }
    case "sync":
    	return func() {
    	    log.Print("Toggling sync state")
    	    syncState = !syncState
    	    toggleImageButton(sw)
    	}
	case "sensorpanelToggle":
		return func() {
			go sendSensorPanelToggle()
		}
    default:
    	return func() {}
    }
}

func sendSensorPanelToggle() {
	report := SensorReport {
		Reporter: "uberswitch",
		Topic: "actions",
		Sensors: map[string]string{ "toggle_screen": "1"},
	}

	requestBody, err := json.Marshal(report)
	if err != nil {
		log.Printf("Failed to create sensor report: %+v", err)
	} else {
		resp, err := http.Post(config.SensorRelayUrl, "application/json", bytes.NewReader(requestBody))
		if err != nil {
			log.Printf("Failed to send sensor report: %+v", err)
		} else if resp.StatusCode != http.StatusOK {
			log.Printf("Failed to send sensor report: %+v", resp)
		}
	}
}

func httpServer() {
    log.Printf("Starting server at %s\n", HttpListenAddr)

    for _, sw := range config.Switches {
        endpoint := fmt.Sprintf("/%s", sw.Name)
        log.Printf("Configuring %s for switch %+v", endpoint, sw)
        handler := getSwitchFunction(sw)
    	http.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
            log.Printf("Actionhandler for %s triggered", endpoint)
            toggleMutex.Lock()
            defer toggleMutex.Unlock()
            handler()
            w.WriteHeader(201)
        })
    }

    if err := http.ListenAndServe(HttpListenAddr, nil); err != nil {
        log.Fatal(err)
    }
}

func initGpio() {
    err := rpio.Open()
    if err != nil {
        panic(err)
    }
}

func blinkGpioPin(pinNumber int) {

	if *noHardware {
	    log.Printf("Would blink %d", pinNumber)
	    return
    }

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

func initImageToggleButton(sw Switch) {
    buttonState := GetButtonState(sw.ButtonIndex, func() ButtonState {
        return ButtonState{sw.ButtonIndex, sw.Images, 0}
    })

    setImageToggleButton(buttonState, getSwitchFunction(sw))
}

func toggleImageButton(sw Switch) {

    if *noHardware {
        log.Printf("Would toggle image button %+v", sw)
        return
    }

	buttonState := GetButtonStateOrPanic(sw.ButtonIndex)

    (*buttonState).imageIndex++
    if buttonState.imageIndex >= len(buttonState.images) {
        (*buttonState).imageIndex = 0
    }
    PersistButtonState(*statePath)

    setImageToggleButton(buttonState, getSwitchFunction(sw))
}

func setImageToggleButton(buttonState *ButtonState, function func()) {
    button, err := buttons.NewImageFileButton(imagePath(buttonState.images[buttonState.imageIndex]))
    if err != nil {
        panic(err)
    }

    button.SetActionHandler(actionhandlers.NewCustomAction(func(button streamdeck.Button) {
        log.Printf("Action handler for %+v triggered", button)
        function()
    }))

    sd.AddButton(buttonState.buttonIndex, button)
}

func initStreamdeck() {
    newSd, err := streamdeck.New()
    if err != nil {
        panic(err)
    }

    sd = newSd

    log.Printf("Found device [%s]\n", sd.GetName())

    initStreamDeckButtons()

    go handleSignals()
}

func filterSwitches(switches []Switch, test func(Switch) bool) (ret []Switch) {
    for _, sw2 := range switches {
        if test(sw2) {
            ret = append(ret, sw2)
        }
    }

    return ret
}

func initStreamDeckButtons() {

	for _, sw := range config.Switches {
        initImageToggleButton(sw)
    }

    go handleSignals()
}

func initMotionSensor(function func(bool)) {

    if config.PhilipsHueSensorUrl != "" {
        go pollMotionSensor(function)
    }
}

type PhilipHueState struct {
    Presence    bool
    Lastupdated string
}

type PhilipsHueResponse struct {
    State PhilipHueState
}

func pollMotionSensor(function func(bool)) {

    var lastPresence = false
    var lastPresenceToFalseChange = time.Now()
    var presenceFalseSent = false

    for {
        response, err := http.Get(config.PhilipsHueSensorUrl)
        if err != nil {
            fmt.Println(err)
        } else {
            if response.StatusCode != 200 {
                fmt.Println("Unexpected response:", response.StatusCode, response.Status)
            } else {
                hueResponse := new(PhilipsHueResponse)
                json.NewDecoder(response.Body).Decode(hueResponse)

                now := time.Now()

                if lastPresence && !hueResponse.State.Presence {
                    lastPresenceToFalseChange = time.Now()
                    presenceFalseSent = false
                } else if !hueResponse.State.Presence && !lastPresence && !presenceFalseSent && now.Sub(lastPresenceToFalseChange) > time.Second*time.Duration(config.MotionSensorThresholdSecs) {
                    function(false)
                    presenceFalseSent = true
                } else if !lastPresence && hueResponse.State.Presence && presenceFalseSent {
                    function(true)
                    presenceFalseSent = false
                }

                lastPresence = hueResponse.State.Presence
            }
            response.Body.Close()
        }

        time.Sleep(10 * time.Second)
    }
}
