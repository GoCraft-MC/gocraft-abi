package abi

// HostCall is a protocol-neutral request for a mutation or side effect.
// Calls are queued by the host and applied by the simulation tick.
type HostCall struct {
	Type   string
	Fields []Value
}

// EffectMessage delivers one line to the player a call names.
//
// The only effect there is today, and it lives in the contract rather than in
// the host because both ends spell it: a plugin asks for it by this string and
// the host dispatches on it. Written twice, the two would agree until somebody
// renamed one — and the failure is a plugin whose messages silently stop
// arriving, since an effect nothing recognises is dropped rather than refused.
//
// Fields are the recipient and the text. The recipient is a whole PlayerRef
// when a native event passes back the one it carried, and a bare sixteen-byte
// uuid when a plugin-defined event names one from its own layout — that event
// has no PlayerRef to pass, its author having declared the fields themselves.
const EffectMessage = "chat.message"