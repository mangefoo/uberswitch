package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "strconv"
)

type ButtonState struct {
    buttonIndex int
    images      []string
    imageIndex  int
}

var states = make(map[int]*ButtonState)

type persistedButtonState struct {
    Index  int
    Images []string
}

type persistedState struct {
    Buttons map[string]persistedButtonState
}

func RestoreButtonState(statePath string) {
    file, err := os.Open(statePath)
    if err != nil {
        panic(err)
    }
    defer file.Close()

    decoder := json.NewDecoder(file)
    persistedState := persistedState{}
    err = decoder.Decode(&persistedState)
    if err != nil {
        log.Fatalf("Failed to parse state file: %+v", err)
        panic(err)
    }

    tmpStates := make(map[int]*ButtonState)
    for key, value := range persistedState.Buttons {
        convIndex, err := strconv.Atoi(key)
        if err != nil {
            log.Fatalf("Failed to parse state index %s: %+v", key, err)
        }

        state := ButtonState{convIndex, value.Images, value.Index}
        tmpStates[convIndex] = &state
    }

    states = tmpStates

    log.Printf("Restored state %+v\n", states)
}

func PersistButtonState(statePath string) {
    toPersist := persistedState { make(map[string]persistedButtonState) }
    for key, state := range states {
        toPersist.Buttons[fmt.Sprintf("%d", key)] = persistedButtonState { state.imageIndex, state.images }
    }

    bytes, err := json.Marshal(toPersist)
    if err != nil {
        log.Printf("Failed to serialize state to JSON: %+v", err)
        return
    }

    ioutil.WriteFile(statePath, bytes, 0644)
}

func GetButtonState(buttonIndex int, init func() ButtonState) *ButtonState {
    if state, ok := states[buttonIndex]; ok {
        return state
    }

    state := init()
    states[buttonIndex] = &state

    return &state
}
