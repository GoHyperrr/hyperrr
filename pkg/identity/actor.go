package identity

import (
	"github.com/GoHyperrr/mdk"
)

// ActorType represents the type of security principal.
type ActorType = mdk.ActorType

const (
	ActorHuman   = mdk.ActorHuman
	ActorAIAgent = mdk.ActorAIAgent
	ActorSystem  = mdk.ActorSystem
)

type Actor = mdk.Actor

type BaseActor = mdk.BaseActor
