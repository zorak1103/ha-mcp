package access

import (
	"reflect"
	"testing"
)

func TestClassifyService_Table(t *testing.T) {
	sys := func(v SystemValue) ServiceClass { return ServiceClass{Kind: ServiceSystem, System: v} }
	kind := func(k ServiceKind) ServiceClass { return ServiceClass{Kind: k} }

	tests := []struct {
		domain, service string
		want            ServiceClass
	}{
		// read
		{"todo", "get_items", kind(ServiceRead)},
		{"calendar", "get_events", kind(ServiceRead)},
		{"weather", "get_forecasts", kind(ServiceRead)},
		// plain control
		{"light", "turn_on", kind(ServiceControl)},
		{"notify", "mobile_app_x", kind(ServiceControl)},
		{"homeassistant", "turn_off", kind(ServiceControl)},
		{"automation", "trigger", kind(ServiceControl)},
		// scripts
		{"script", "turn_on", kind(ServiceControl)},
		{"script", "turn_off", kind(ServiceControl)},
		{"script", "toggle", kind(ServiceControl)},
		{"script", "reload", sys(SystemReload)},
		{"script", "morning_routine", kind(ServiceScriptCall)},
		// reload
		{"automation", "reload", sys(SystemReload)},
		{"scene", "reload", sys(SystemReload)},
		{"homeassistant", "reload_all", sys(SystemReload)},
		{"homeassistant", "reload_core_config", sys(SystemReload)},
		{"homeassistant", "reload_config_entry", sys(SystemReload)},
		// restart
		{"homeassistant", "restart", sys(SystemRestart)},
		{"homeassistant", "stop", sys(SystemRestart)},
		{"hassio", "host_reboot", sys(SystemRestart)},
		{"hassio", "host_shutdown", sys(SystemRestart)},
		// addons, backup
		{"hassio", "addon_start", sys(SystemAddons)},
		{"hassio", "addon_stdin", sys(SystemAddons)},
		{"backup", "create", sys(SystemBackup)},
		{"backup", "create_automatic", sys(SystemBackup)},
		{"hassio", "backup_full", sys(SystemBackup)},
		{"hassio", "restore_full", sys(SystemBackup)},
		// updates, statistics, logs
		{"update", "install", sys(SystemUpdates)},
		{"update", "skip", sys(SystemUpdates)},
		{"recorder", "purge", sys(SystemStatistics)},
		{"logger", "set_level", sys(SystemLogs)},
		{"system_log", "write", sys(SystemLogs)},
		// code execution and misc homeassistant
		{"shell_command", "x", sys(SystemHomeAssistant)},
		{"python_script", "run", sys(SystemHomeAssistant)},
		{"pyscript", "anything", sys(SystemHomeAssistant)},
		{"rest_command", "call_api", sys(SystemHomeAssistant)},
		{"homeassistant", "set_location", sys(SystemHomeAssistant)},
		// unknown hassio services fail closed
		{"hassio", "app_stdin", sys(SystemHomeAssistant)},
		// configure
		// (scene.create, group.set and group.remove: see TestClassifyService_ConfigureEntityDataKeys)
		{"input_select", "set_options", kind(ServiceConfigure)},
		// indirect objects named in the call data
		{"scene", "apply", ServiceClass{Kind: ServiceControl, EntityDataKeys: []string{"entities"}}},
		{"media_player", "join", ServiceClass{Kind: ServiceControl, EntityDataKeys: []string{"group_members"}}},
		// plain homeassistant services stay control
		{"homeassistant", "toggle", kind(ServiceControl)},
		{"homeassistant", "update_entity", kind(ServiceControl)},
		// restore_* is an intentional extension of the brief's backup_* rule
		{"hassio", "restore_partial", sys(SystemBackup)},
		{"HASSIO", "Host_Reboot", sys(SystemRestart)},
		// integrations that reach arbitrary entities or hardware without a matchable object
		{"conversation", "process", sys(SystemHomeAssistant)},
		{"Conversation", "Reload", sys(SystemHomeAssistant)},
		{"mqtt", "publish", sys(SystemHomeAssistant)},
		{"MQTT", "Dump", sys(SystemHomeAssistant)},
		{"zha", "issue_zigbee_cluster_command", sys(SystemHomeAssistant)},
		{"zwave_js", "set_value", sys(SystemHomeAssistant)},
		{"ZWAVE_JS", "Multicast_Set_Value", sys(SystemHomeAssistant)},
	}
	for _, tt := range tests {
		t.Run(tt.domain+"."+tt.service, func(t *testing.T) {
			if got := ClassifyService(tt.domain, tt.service); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClassifyService(%q, %q) = %+v, want %+v", tt.domain, tt.service, got, tt.want)
			}
		})
	}
}

func TestClassifyService_ConfigureEntityDataKeys(t *testing.T) {
	tests := []struct {
		domain, service string
		want            ServiceClass
	}{
		{"scene", "create", ServiceClass{Kind: ServiceConfigure, ObjectFromData: "scene_id", EntityDataKeys: []string{"entities", "snapshot_entities"}}},
		{"group", "set", ServiceClass{Kind: ServiceConfigure, ObjectFromData: "object_id", EntityDataKeys: []string{"entities", "add_entities"}}},
		{"group", "remove", ServiceClass{Kind: ServiceConfigure, ObjectFromData: "object_id"}},
	}
	for _, tt := range tests {
		if got := ClassifyService(tt.domain, tt.service); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ClassifyService(%q, %q) = %+v, want %+v", tt.domain, tt.service, got, tt.want)
		}
	}
}

func TestClassifyService_EntityDataKeysAreNotShared(t *testing.T) {
	first := ClassifyService("scene", "apply")
	first.EntityDataKeys[0] = "tampered"
	if got := ClassifyService("scene", "apply").EntityDataKeys; !reflect.DeepEqual(got, []string{"entities"}) {
		t.Errorf("caller mutation leaked into the table: %v", got)
	}
}

func TestClassifyService_CaseAndWhitespaceCannotBypass(t *testing.T) {
	tests := []struct {
		domain, service string
		want            ServiceClass
	}{
		{"Shell_Command", "X", ServiceClass{Kind: ServiceSystem, System: SystemHomeAssistant}},
		{" homeassistant ", " RESTART\t", ServiceClass{Kind: ServiceSystem, System: SystemRestart}},
		{"AUTOMATION", "Reload", ServiceClass{Kind: ServiceSystem, System: SystemReload}},
		{"Todo", "Get_Items", ServiceClass{Kind: ServiceRead}},
		{"SCRIPT", "My_Script", ServiceClass{Kind: ServiceScriptCall}},
		{"Scene", "CREATE", ServiceClass{Kind: ServiceConfigure, ObjectFromData: "scene_id", EntityDataKeys: []string{"entities", "snapshot_entities"}}},
		{"Scene", " Apply", ServiceClass{Kind: ServiceControl, EntityDataKeys: []string{"entities"}}},
	}
	for _, tt := range tests {
		if got := ClassifyService(tt.domain, tt.service); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ClassifyService(%q, %q) = %+v, want %+v", tt.domain, tt.service, got, tt.want)
		}
	}
}

func TestClassifyService_MalformedFailsClosed(t *testing.T) {
	want := ServiceClass{Kind: ServiceSystem, System: SystemAll}
	for _, in := range [][2]string{{"", ""}, {"", "turn_on"}, {"light", ""}, {"  ", "x"}, {"light", " "}} {
		if got := ClassifyService(in[0], in[1]); !reflect.DeepEqual(got, want) {
			t.Errorf("ClassifyService(%q, %q) = %+v, want %+v", in[0], in[1], got, want)
		}
	}
}

// staticHandlerCommands is the set of WebSocket command names that handlers send
// through the Client.SendHACSCommand passthrough (the only passthrough taking an
// arbitrary command name). It was derived by grepping every non-test call of
// SendHACSCommand( under internal/handlers; internal/homeassistant only forwards
// the name. Dedicated Client methods (registries, call_service, ...) are
// classified by the wrapper per method, not by command name.
var staticHandlerCommands = map[string]CommandClass{
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

// Only trace/list and trace/get carry a (domain, config id) pair that must be
// resolved through the registry; no other command may expose these fields.
func TestClassifyWSCommand_TraceExposesDomainAndConfigIDOnly(t *testing.T) {
	for cmd := range staticHandlerCommands {
		got, ok := ClassifyWSCommand(cmd)
		if !ok {
			t.Fatalf("ClassifyWSCommand(%q): not classified", cmd)
		}
		isTrace := cmd == "trace/list" || cmd == "trace/get"
		if isTrace {
			if got.DomainArg != "domain" || got.ConfigIDArg != "item_id" {
				t.Errorf("%s: DomainArg/ConfigIDArg = %q/%q, want domain/item_id", cmd, got.DomainArg, got.ConfigIDArg)
			}
			continue
		}
		if got.DomainArg != "" || got.ConfigIDArg != "" {
			t.Errorf("%s: unexpected DomainArg/ConfigIDArg %q/%q", cmd, got.DomainArg, got.ConfigIDArg)
		}
	}
}

func TestClassifyWSCommand_EveryStaticHandlerCommand(t *testing.T) {
	if len(staticHandlerCommands) != 17 {
		t.Fatalf("expected 17 static handler commands, have %d", len(staticHandlerCommands))
	}
	for cmd, want := range staticHandlerCommands {
		got, ok := ClassifyWSCommand(cmd)
		if !ok {
			t.Errorf("ClassifyWSCommand(%q): not classified", cmd)
			continue
		}
		if got != want {
			t.Errorf("ClassifyWSCommand(%q) = %+v, want %+v", cmd, got, want)
		}
	}
}

func TestClassifyWSCommand_UnknownIsRefused(t *testing.T) {
	for _, cmd := range []string{"", "foo/bar", "hacs", "hacs/", "HACS/info", " hacs/info", "hacs/info ", "call_service", "trace", "hacs/repository/download/extra"} {
		got, ok := ClassifyWSCommand(cmd)
		if ok || got != (CommandClass{}) {
			t.Errorf("ClassifyWSCommand(%q) = %+v, %v; want zero, false", cmd, got, ok)
		}
	}
}

func TestParseTarget_Forms(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want TargetSpec
	}{
		{"nil", nil, TargetSpec{}},
		{"empty", map[string]any{}, TargetSpec{}},
		{"string", map[string]any{"entity_id": "light.a"}, TargetSpec{EntityIDs: []string{"light.a"}}},
		{"any list", map[string]any{"entity_id": []any{"light.a", "light.b"}}, TargetSpec{EntityIDs: []string{"light.a", "light.b"}}},
		{"string list", map[string]any{"entity_id": []string{"light.a", "light.b"}}, TargetSpec{EntityIDs: []string{"light.a", "light.b"}}},
		{"comma string", map[string]any{"entity_id": "light.a, light.b ,,"}, TargetSpec{EntityIDs: []string{"light.a", "light.b"}}},
		{"case and space", map[string]any{"entity_id": "  Light.A "}, TargetSpec{EntityIDs: []string{"light.a"}}},
		{"all", map[string]any{"entity_id": "all"}, TargetSpec{All: true}},
		{"ALL padded", map[string]any{"entity_id": " ALL "}, TargetSpec{All: true}},
		{"all inside list", map[string]any{"entity_id": []any{"light.a", "all"}}, TargetSpec{EntityIDs: []string{"light.a"}, All: true}},
		{"none matches nothing", map[string]any{"entity_id": "none"}, TargetSpec{}},
		{"device", map[string]any{"device_id": "abc"}, TargetSpec{DeviceIDs: []string{"abc"}}},
		{"area", map[string]any{"area_id": []any{"kitchen", "Hall"}}, TargetSpec{AreaIDs: []string{"kitchen", "hall"}}},
		{"floor", map[string]any{"floor_id": "ground"}, TargetSpec{FloorIDs: []string{"ground"}}},
		{"label", map[string]any{"label_id": "a,b"}, TargetSpec{LabelIDs: []string{"a", "b"}}},
		{"nested target merged", map[string]any{
			"entity_id": "light.a",
			"target":    map[string]any{"entity_id": []any{"light.b"}, "area_id": "kitchen"},
		}, TargetSpec{EntityIDs: []string{"light.a", "light.b"}, AreaIDs: []string{"kitchen"}}},
		{"nested all", map[string]any{"target": map[string]any{"entity_id": "all"}}, TargetSpec{All: true}},
		{"duplicates removed", map[string]any{
			"entity_id": "light.a",
			"target":    map[string]any{"entity_id": "LIGHT.A"},
		}, TargetSpec{EntityIDs: []string{"light.a"}}},
		{"unrelated keys ignored", map[string]any{"brightness": 3, "entity_id": "light.a"}, TargetSpec{EntityIDs: []string{"light.a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTarget(tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTarget = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseTarget_MalformedIsFlagged(t *testing.T) {
	tests := map[string]map[string]any{
		"number":          {"entity_id": 5},
		"map":             {"entity_id": map[string]any{"a": "b"}},
		"nil":             {"entity_id": nil},
		"bool":            {"area_id": true},
		"mixed list":      {"entity_id": []any{"light.a", 7}},
		"nested non-map":  {"target": "light.a"},
		"nested nil":      {"target": nil},
		"nested bad type": {"target": map[string]any{"device_id": 1.5}},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if got := ParseTarget(data); !got.Malformed {
				t.Errorf("ParseTarget(%v) = %+v, want Malformed", data, got)
			}
		})
	}
}

func TestParseTarget_ValidIsNotMalformed(t *testing.T) {
	got := ParseTarget(map[string]any{"entity_id": []any{"light.a"}, "target": map[string]any{"area_id": "k"}})
	if got.Malformed {
		t.Errorf("unexpected Malformed: %+v", got)
	}
}

func TestEntityIDsFromData(t *testing.T) {
	tests := []struct {
		name          string
		data          map[string]any
		keys          []string
		want          []string
		wantMalformed bool
	}{
		{"no keys", map[string]any{"entities": "light.a"}, nil, nil, false},
		{"absent key", map[string]any{"x": 1}, []string{"entities"}, nil, false},
		{"nil data", nil, []string{"entities"}, nil, false},
		{"map keys are the ids", map[string]any{"entities": map[string]any{"Lock.Front_Door": "unlocked", "light.a": map[string]any{"state": "on"}}}, []string{"entities"}, []string{"light.a", "lock.front_door"}, false},
		{"list", map[string]any{"entities": []any{"light.a", " Light.B "}}, []string{"entities"}, []string{"light.a", "light.b"}, false},
		{"string list", map[string]any{"entities": []string{"light.a"}}, []string{"entities"}, []string{"light.a"}, false},
		{"string", map[string]any{"entities": "light.a"}, []string{"entities"}, []string{"light.a"}, false},
		{"comma string", map[string]any{"entities": "light.a, light.b,,"}, []string{"entities"}, []string{"light.a", "light.b"}, false},
		{"several keys deduped", map[string]any{"entities": "light.a", "snapshot_entities": []any{"LIGHT.A", "light.c"}}, []string{"entities", "snapshot_entities"}, []string{"light.a", "light.c"}, false},
		{"empty map", map[string]any{"entities": map[string]any{}}, []string{"entities"}, nil, false},
		{"number", map[string]any{"entities": 3}, []string{"entities"}, nil, true},
		{"bool", map[string]any{"entities": true}, []string{"entities"}, nil, true},
		{"nil value", map[string]any{"entities": nil}, []string{"entities"}, nil, true},
		{"mixed list keeps good ids", map[string]any{"entities": []any{"light.a", 7}}, []string{"entities"}, []string{"light.a"}, true},
		{"bad key does not hide good key", map[string]any{"entities": 1, "add_entities": "light.z"}, []string{"entities", "add_entities"}, []string{"light.z"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, malformed := EntityIDsFromData(tt.data, tt.keys)
			if !reflect.DeepEqual(got, tt.want) || malformed != tt.wantMalformed {
				t.Errorf("EntityIDsFromData = %v, %v; want %v, %v", got, malformed, tt.want, tt.wantMalformed)
			}
		})
	}
}
