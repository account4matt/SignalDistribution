// Copyright 2014 liveease.com. All rights reserved.

// Package signal implements the signal station that can relay signals.
//
// A station contains channels, relay clients, recorder clients.
// A channel contains some end-clients.
package signal

import (
	"saassoft.net/signaldistribution/base"
	"time"
)

// Signal represents a signal object.
type Signal struct {
	ID   string
	PID  string
	Type base.SignalType
	Text string
}

// SignalPack is the package of singal for being transmitted between station.
type SignalPack struct {
	Signal   Signal
	CID      string
	Time     time.Time
	Stations []string
}
