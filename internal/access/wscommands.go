package access

// CommandClass is the classification of a WebSocket command sent through the
// client's command passthrough.
//
// EntityArg names the command-data argument holding a full entity id the read
// refers to (update/release_notes); empty otherwise.
//
// DomainArg and ConfigIDArg (trace/list and trace/get only, empty for every
// other command) name the arguments holding the object's domain and its config
// id. The config id is NOT an entity id: for automations the entity id is a slug
// of the alias, so the wrapper must resolve (DomainArg value, ConfigIDArg value)
// to an entity through the registry and never build "domain.configid". It must
// refuse when the DomainArg value is absent or not automation or script, and
// when the ConfigIDArg value is absent (a domain-wide listing) it must require
// read on every entity of that domain or filter the result accordingly.
type CommandClass struct {
	Kind        ServiceKind
	System      SystemValue
	EntityArg   string
	DomainArg   string
	ConfigIDArg string
}

// wsCommands lists every command handlers send through the passthrough. Only
// ServiceRead and ServiceSystem occur; anything absent is unknown.
var wsCommands = map[string]CommandClass{
	"blueprint/list":                {Kind: ServiceRead},
	"blueprint/import":              {Kind: ServiceSystem, System: SystemBlueprints},
	"hacs/info":                     {Kind: ServiceRead},
	"hacs/repositories/list":        {Kind: ServiceRead},
	"hacs/repository/info":          {Kind: ServiceRead},
	"hacs/repository/releases":      {Kind: ServiceRead},
	"hacs/repository/release_notes": {Kind: ServiceRead},
	"hacs/critical/list":            {Kind: ServiceRead},
	"hacs/repository/download":      {Kind: ServiceSystem, System: SystemHACS},
	"hacs/repository/remove":        {Kind: ServiceSystem, System: SystemHACS},
	"hacs/repositories/add":         {Kind: ServiceSystem, System: SystemHACS},
	"hacs/repositories/remove":      {Kind: ServiceSystem, System: SystemHACS},
	"hacs/repository/refresh":       {Kind: ServiceSystem, System: SystemHACS},
	"hacs/repository/beta":          {Kind: ServiceSystem, System: SystemHACS},
	"trace/list":                    {Kind: ServiceRead, DomainArg: "domain", ConfigIDArg: "item_id"},
	"trace/get":                     {Kind: ServiceRead, DomainArg: "domain", ConfigIDArg: "item_id"},
	"update/release_notes":          {Kind: ServiceRead, EntityArg: "entity_id"},
}

// ClassifyWSCommand classifies a passthrough WebSocket command. The name is
// matched exactly (Home Assistant command names are case-sensitive, so a
// variant would be rejected by HA anyway). ok is false for an unknown command;
// the caller must refuse it.
func ClassifyWSCommand(cmd string) (CommandClass, bool) {
	c, ok := wsCommands[cmd]
	return c, ok
}
