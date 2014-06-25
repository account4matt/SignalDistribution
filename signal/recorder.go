// Copyright 2014 liveease.com. All rights reserved.

package signal

import (
	"code.google.com/p/go.net/websocket"
	"time"
)

// Recorder represents a signals recorder client,it is kind of participant.
// The recorder client listens to a station, when one of the station's channels receives a signal,
// it sends the signal to the recorder server.
type Recorder struct {
	Info      ParticipantStruct
	Station   *Station
	RemoteSID string
	Time      time.Time
}

// StartBroadcast does not work in recorder client.
func (this *Recorder) StartBroadcast() {
	for {
		var signal SignalPack
		if err := websocket.JSON.Receive(this.Info.Remote.Conn, &signal); err != nil {
			return
		}
	}
}

// StartListen starts to listen the station, once a signal is received, sends the signal to the recorder server.
func (this *Recorder) StartListen() {
	for signal := range this.Info.Signals {
		if signal == nil {
			return
		}

		if err := websocket.JSON.Send(this.Info.Remote.Conn, signal); err != nil {
			break
		}
	}
}

// Relay does not work in recorder client.
func (this *Recorder) Relay(signal *SignalPack) error {
	return nil
}

// PushSignal pushes a signal to the recorder client.
func (this *Recorder) PushSignal(signal *SignalPack) error {
	this.Info.Signals <- signal
	return nil
}

//  PushSignal pushes signals to the recorder client.
func (this *Recorder) PushSignals(signals []*SignalPack) {
	for _, signal := range signals {
		this.PushSignal(signal)
	}
}

// Close closes the recorder client,and release the resources of the recorder client.
func (this *Recorder) Release() {
	close(this.Info.Signals)
	_ = this.Info.Remote.Conn.Close()
}
