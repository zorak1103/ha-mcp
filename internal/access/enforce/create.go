package enforce

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/access"
)

// Creating a config whose id does not exist yet. Home Assistant derives the
// new entity's id (predictEntityID), so the create is decided on that id: it
// needs configure and create there. When that id is already taken by another
// entity, HA would silently pick <id>_2, an entity the policy never decided
// on, so the create is refused instead with errUnavailable. That error is the
// same whether the taken entity is hidden or visible, and it is not recorded
// as a denial (it is not a policy decision).

// Capabilities that refine configure, named in denials.
const (
	capCreate access.Capability = "create"
	capDelete access.Capability = "delete"
)

// errUnavailable is the single neutral answer when a config or entity id is
// already taken by something the caller may not overwrite or cannot see.
func errUnavailable(id string) error {
	return fmt.Errorf("id %q is not available", id)
}

// checkNew decides the create of a config id the registry does not know and
// returns the entity id the new config will get.
//
// A config id present in the config file without a registry entry (created
// but not yet loaded, or not loadable) cannot be mapped to its entity: it is
// answered with errUnavailable, as a hidden config id is, so an upsert can
// never overwrite a config the policy has not decided on.
func (c *client) checkNew(ctx context.Context, w configWrite) (string, error) {
	inFile, err := c.inner.ConfigFileEntryExists(ctx, w.domain, w.id)
	if err != nil {
		return "", c.undecidable(w.method, err)
	}
	if inFile {
		return "", errUnavailable(w.id)
	}
	predicted := c.predictEntityID(w.domain, w.name)
	if predicted == "" {
		return "", c.refuse(w.method)
	}
	taken, err := c.entityTaken(ctx, predicted)
	if err != nil {
		return "", c.undecidable(w.method, err)
	}
	if taken {
		return "", errUnavailable(predicted)
	}
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return "", c.undecidable(w.method, err)
	}
	g := grant(predicted)
	switch {
	case !g.Configure:
		return "", c.denyObject(predicted, access.CapConfigure)
	case !g.Create:
		return "", c.denyObject(predicted, capCreate)
	}
	return predicted, nil
}
