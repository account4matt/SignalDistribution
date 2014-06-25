// Copyright 2014 liveease.com. All rights reserved.

package signal

import (
	"saassoft.net/signaldistribution/base"
)

// Participant defines interfaces of participant in the station.
// Relay (other station), Recorder client, Client are kinds of participant.
type Participant interface {
	StartBroadcast()
	StartListen()
	Relay(signal *SignalPack)
	PushSignal(signal *SignalPack)
	PushSignals(signals []*SignalPack)
	Release()
}

// ParticipantStruct describes the base informations of a participant.
type ParticipantStruct struct {
	UPID    string
	PID     string
	Signals chan *SignalPack
	Remote  *base.RemoteInfo
}
