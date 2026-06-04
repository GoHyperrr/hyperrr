package identity

import (
	"github.com/GoHyperrr/mdk"
)

// Re-export/alias mdk actor types for backward compatibility inside hyperrr.
type ActorType = mdk.ActorType

const (
	ActorHuman   = mdk.ActorHuman
	ActorAIAgent = mdk.ActorAIAgent
	ActorSystem  = mdk.ActorSystem
)

type Actor = mdk.Actor

