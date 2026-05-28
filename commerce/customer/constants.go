package customer

// Personas.
const (
	PersonaWhale      = "WHALE"
	PersonaGold       = "GOLD"
	PersonaFrustrated = "FRUSTRATED"
	PersonaRegular    = "REGULAR"
	PersonaNewbie     = "NEWBIE"
)

// Workflow task names.
const (
	TaskCalculatePersona = "customer.calculate_persona"
	TaskUpdatePersona    = "customer.update_persona"
	TaskUpdateDetails    = "customer.update_details"
)

// Event types.
const (
	EventCustomerProfileCreated = "customer.profile_created"
)
