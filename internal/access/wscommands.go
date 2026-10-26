package access

// CommandClass is the classification of a WebSocket command sent through the
// client's command passthrough. EntityArg names the command-data argument that
// identifies the object a read refers to (empty when the command is not tied to
// one object). Note that trace/list and trace/get carry the bare automation or
// script config id in item_id (together with domain), not a full entity id.
type CommandClass struct {
	Kind      ServiceKind
	System    SystemValue
	EntityArg string
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
	"trace/list":                    {Kind: ServiceRead, EntityArg: "item_id"},
	"trace/get":                     {Kind: ServiceRead, EntityArg: "item_id"},
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
