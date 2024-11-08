// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package components provides application components, i.e. coordinators,
// workers, and common definitions shared by them.
package components

import (
	"strings"

	cmpt "github.com/coatyio/dda-examples/compute/computation"
)

const (
	ActionTypeCompute  = "ddaexmpls.compute.pcomp" // Action type of partial computation
	ActionShareCompute = "pcompute"                // shared subscription for partial computation
)

const (
	ActionEventTypeAnnounceCoordinator = "ddaexmpls.compute.announceCoordinator" // Action and Event type of coordinator announcements
	EventTypeAnnounceWorker            = "ddaexmpls.compute.announceWorker"      // Event type of worker announcements
)

var (
	DataAnnounceJoin  cmpt.BinaryData = []byte("HELLO") // announcement data on joining/being alive
	DataAnnounceLeave cmpt.BinaryData = []byte("BYE")   // announcement data on leaving
)

// ComponentRole represents different roles of application components.
type ComponentRole int

const (
	RoleUndefined   ComponentRole = iota // undefined role
	RoleCoordinator                      // coordinator role
	RoleWorker                           // worker role
)

// String returns a human-readable format of a ComponentRole.
func (r ComponentRole) String() string {
	switch r {
	case RoleCoordinator:
		return "coordinator"
	case RoleWorker:
		return "worker"
	default:
		return "undefined"
	}
}

// ParseComponentRole parses the given role string into a ComponentRole.
func ParseComponentRole(r string) ComponentRole {
	switch r {
	case "coordinator":
		return RoleCoordinator
	case "worker":
		return RoleWorker
	default:
		return RoleUndefined
	}
}

// UuidShort returns the first part of a string in UUID v4 format; otherwise the
// complete string is returned.
func UuidShort(uuid string) string {
	i := strings.Index(uuid, "-")
	if i != -1 {
		return uuid[:i]
	}
	return uuid
}
