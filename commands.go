package silo

// Command types for ingress triage signaling.
const (
	CommandIdentity  = "IDENTITY"
	CommandSync      = "SYNC"
	CommandStructure = "STRUCTURE"
	CommandWrite     = "WRITE"
	CommandRead      = "READ"
	CommandJump      = "JUMP"
	CommandObserve   = "OBSERVE"
	CommandRadar     = "RADAR"
)

// Priority hints for traffic optimization.
const (
	PriorityHigh   = "1"
	PriorityNormal = "2"
	PriorityLow    = "3"
)
