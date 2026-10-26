package enforce

import (
	"context"
	"regexp"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Resolution of stored automation, script and scene configs to the entities
// the policy decides on. The config id is never trusted to name the entity:
// it is looked up in the unfiltered entity registry (platform plus unique_id),
// and a create is decided on the entity id Home Assistant will derive.

// configIDRE is the accepted shape of an automation or scene config id (the
// handlers generate slugs, the HA UI numeric strings). Anything else, which
// would reach the REST path verbatim, is refused.
var configIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// validConfigID reports whether id has the shape accepted for the domain: a
// script key is the object id of its entity, so it must be a slug.
func validConfigID(domain, id string) bool {
	if domain == domainScript {
		return slugRE.MatchString(id)
	}
	return configIDRE.MatchString(id)
}

// entityForConfigID resolves a stored config to its entity through the
// unfiltered registry: an entry of the domain's platform whose unique_id is
// the config id (automation config id, script key, scene config id). exists
// is false when no entry matches; entityID is then <domain>.<id>, which is the
// entity id of a script but only a guess for automations and scenes. A
// registry fetch failure is returned, never read as "does not exist".
// Scene.EntityID and similar client-reported ids are never consulted.
func (c *client) entityForConfigID(ctx context.Context, kind, id string) (entityID string, exists bool, err error) {
	data, err := c.snap.get(ctx)
	if err != nil {
		return "", false, err
	}
	entityID, exists = data.entityForConfigID(kind, id)
	return entityID, exists, nil
}

// predictEntityID is the entity id Home Assistant derives for a new config:
// automation.<slug(alias)>, script.<key>, scene.<slug(name)>. It returns ""
// when no plain valid entity id can be derived (empty or unsluggable name);
// the caller refuses then.
func (c *client) predictEntityID(kind, name string) string {
	var object string
	switch kind {
	case domainScript:
		object = name
	case domainAutomation, domainScene:
		object = homeassistant.Slugify(name)
	default:
		return ""
	}
	id := kind + "." + object
	if object == "" || !isPlainEntityID(id) {
		return ""
	}
	return id
}

// entityTaken reports whether entityID already names an entity, in the
// unfiltered registry or the state machine, hidden or not.
func (c *client) entityTaken(ctx context.Context, entityID string) (bool, error) {
	data, err := c.snap.get(ctx)
	if err != nil {
		return false, err
	}
	if _, ok := data.entity(entityID); ok {
		return true, nil
	}
	states, err := (&stateSet{}).load(ctx, c.inner)
	if err != nil {
		return false, err
	}
	return states[strings.ToLower(entityID)], nil
}

// entityForRegistryID resolves an entity-registry id (as named by a device
// action) to its entity id through the unfiltered registry.
func (d *snapData) entityForRegistryID(registryID string) (string, bool) {
	for _, id := range d.entityIDs {
		if e := d.entities[id]; e.ID != "" && e.ID == registryID {
			return id, true
		}
	}
	return "", false
}
