package bot

// daveReply is the unconditional response. The Python bot special-cased one
// user ("bigsamspintofwine#0" -> "fuck you Steve"); that discriminator-based
// check is dead (Discord dropped discriminators) and was cut in T07 — /dave now
// replies the same for every caller, with no user check and no config key.
const daveReply = "fuck you Dave"

func handleDave(r Responder) error {
	return r.Respond(daveReply)
}
