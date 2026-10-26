package enforce

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// absentWorld returns a wrapper over the same home without the hidden objects,
// under a policy that hides nothing relevant: what HA answers for ids that are
// simply missing.
func absentWorld(t *testing.T) (*client, *readFake) {
	t.Helper()
	absent := buildFake(false)
	absent.extractFn = absent.echoExtract
	other := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"zzz.*"}}, Allow: config.CapList{"none"}}},
	})
	w, _, _ := wrapFake(t, other, absent)
	return w, absent
}

// Ids that look like twins of an earlier, predictable scheme must be echoed
// back unchanged next to a hidden id: nothing merges, nothing disappears.
func TestTwins_CallerSuppliedTwinShapesAreEchoedUnchanged(t *testing.T) {
	full := buildFake(true)
	full.extractFn = func(tg homeassistant.Target) *homeassistant.ExtractFromTargetResult {
		return reverseLists(full.echoExtract(tg))
	}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	wAbsent, absent := absentWorld(t)
	absent.extractFn = func(tg homeassistant.Target) *homeassistant.ExtractFromTargetResult {
		return reverseLists(absent.echoExtract(tg))
	}

	in := homeassistant.Target{
		EntityID: []string{"light.ha_mcp_absent_0", "light.secret", "light.gone"},
		DeviceID: []string{"ha_mcp_absent_device_0", "dev_hidden"},
		AreaID:   []string{"ha_mcp_absent_area_0"},
	}
	got, err := w.ExtractFromTarget(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := wAbsent.ExtractFromTarget(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
		t.Errorf("answer differs from the all-missing answer\n got %+v\nwant %+v", got, want)
	}
	if !slices.Equal(got.ReferencedEntities, []string{"light.gone", "light.ha_mcp_absent_0", "light.secret"}) {
		t.Errorf("an id vanished or merged: %v", got.ReferencedEntities)
	}
	if !slices.Equal(got.MissingDevices, []string{"dev_hidden", "ha_mcp_absent_device_0"}) ||
		!slices.Equal(got.MissingAreas, []string{"ha_mcp_absent_area_0"}) {
		t.Errorf("missing lists = %v / %v", got.MissingDevices, got.MissingAreas)
	}
	for _, tg := range full.targets {
		for _, id := range slices.Concat(tg.EntityID, tg.DeviceID, tg.AreaID) {
			if strings.Contains(id, "absent") && !strings.Contains(id, "ha_mcp_") {
				t.Errorf("unexpected id %q", id)
			}
			if id == "light.ha_mcp_absent_0" || id == "ha_mcp_absent_device_0" || id == "ha_mcp_absent_area_0" ||
				strings.Contains(id, "secret") || id == "dev_hidden" {
				t.Errorf("id %q reached the inner client", id)
			}
		}
	}
}

func setNonces(t *testing.T, nonces ...string) {
	t.Helper()
	orig := newNonce
	i := 0
	newNonce = func() string {
		n := nonces[min(i, len(nonces)-1)]
		i++
		return n
	}
	t.Cleanup(func() { newNonce = orig })
}

func TestTwins_NonceClashRegenerates(t *testing.T) {
	setNonces(t, "aaaa1111", "bbbb2222")
	full := buildFake(true)
	// A real entity that carries the first nonce's twin pattern.
	full.registry = append(full.registry, homeassistant.EntityRegistryEntry{EntityID: "light.ha_mcp_aaaa1111_0"})
	full.extractFn = full.echoExtract
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)

	got, err := w.ExtractFromTarget(ctx, homeassistant.Target{EntityID: []string{"light.secret"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.targets) != 1 || len(full.targets[0].EntityID) != 1 || !strings.Contains(full.targets[0].EntityID[0], "ha_mcp_bbbb2222_") {
		t.Errorf("inner target = %+v, want a twin built from the second nonce", full.targets)
	}
	if !slices.Equal(got.ReferencedEntities, []string{"light.secret"}) {
		t.Errorf("result = %+v", got)
	}
}

func TestTwins_NonceClashOnRequestIDAndExhaustion(t *testing.T) {
	setNonces(t, "aaaa1111")
	full := buildFake(true)
	full.extractFn = full.echoExtract
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	_, err := w.ExtractFromTarget(ctx, homeassistant.Target{EntityID: []string{"light.secret", "light.ha_mcp_aaaa1111_0"}}, nil)
	if err == nil {
		t.Fatal("a nonce that clashes on every retry must fail closed")
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "snapshot") || strings.Contains(msg, "nonce") {
		t.Errorf("error leaks internals: %q", err)
	}
	if tr.First() == nil {
		t.Error("the refusal must be recorded")
	}
	if len(full.targets) != 0 {
		t.Error("inner was called")
	}
}

func TestExtractFromTarget_OutputIsCanonicallySorted(t *testing.T) {
	full := buildFake(true)
	full.extractFn = func(tg homeassistant.Target) *homeassistant.ExtractFromTargetResult {
		return reverseLists(full.echoExtract(tg))
	}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	wAbsent, absent := absentWorld(t)
	absent.extractFn = func(tg homeassistant.Target) *homeassistant.ExtractFromTargetResult {
		return reverseLists(absent.echoExtract(tg))
	}

	in := homeassistant.Target{
		EntityID: []string{"light.zzz_missing", "light.secret", "light.aaa_missing", "light.kitchen"},
		DeviceID: []string{"dev_zzz", "dev_hidden", "dev_aaa"},
		LabelID:  []string{"lbl_z", "lbl_a"},
	}
	got, err := w.ExtractFromTarget(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := wAbsent.ExtractFromTarget(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%#v", got) != fmt.Sprintf("%#v", want) {
		t.Errorf("hidden and missing ids must give one canonical answer\n got %#v\nwant %#v", got, want)
	}
	checks := map[string][]string{
		"entities":       got.ReferencedEntities,
		"devices":        got.ReferencedDevices,
		"areas":          got.ReferencedAreas,
		"missingDevices": got.MissingDevices,
		"missingAreas":   got.MissingAreas,
		"missingFloors":  got.MissingFloors,
		"missingLabels":  got.MissingLabels,
	}
	for name, l := range checks {
		if l == nil {
			t.Errorf("%s: nil, want a non-nil list", name)
		}
		if !slices.IsSorted(l) || len(slices.Compact(slices.Clone(l))) != len(l) {
			t.Errorf("%s = %v, want ascending and de-duplicated", name, l)
		}
	}
	if !slices.Equal(got.ReferencedEntities, []string{"light.aaa_missing", "light.kitchen", "light.secret", "light.zzz_missing"}) {
		t.Errorf("entities = %v", got.ReferencedEntities)
	}
	if !slices.Equal(got.MissingDevices, []string{"dev_aaa", "dev_hidden", "dev_zzz"}) || !slices.Equal(got.MissingLabels, []string{"lbl_a", "lbl_z"}) {
		t.Errorf("missing = %v / %v", got.MissingDevices, got.MissingLabels)
	}
}

func TestExtractFromTarget_NonHidingPolicyKeepsHAOrder(t *testing.T) {
	full := buildFake(true)
	full.extractFn = func(tg homeassistant.Target) *homeassistant.ExtractFromTargetResult {
		return reverseLists(full.echoExtract(tg))
	}
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: config.CapList{"control"}}},
	})
	w, _, ctx := wrapFake(t, p, full)
	got, err := w.ExtractFromTarget(ctx, homeassistant.Target{EntityID: []string{"light.a", "light.b"}}, nil)
	if err != nil || !slices.Equal(got.ReferencedEntities, []string{"light.b", "light.a"}) {
		t.Errorf("a policy that hides nothing must pass through: %+v, %v", got, err)
	}
}

func TestTargetErrors_DoNotEchoTwins(t *testing.T) {
	hiddenIDs := []string{"light.secret0", "light.secret1", "light.secret2", "light.secret3", "light.secret4", "light.secret5",
		"light.secret6", "light.secret7", "light.secret8", "light.secret9", "light.secret10"}
	joined := strings.Join(hiddenIDs, ",")
	calls := map[string]func(c homeassistant.Client, ctx context.Context, tg homeassistant.Target) error{
		"extract": func(c homeassistant.Client, ctx context.Context, tg homeassistant.Target) error {
			_, err := c.ExtractFromTarget(ctx, tg, nil)
			return err
		},
		"triggers": func(c homeassistant.Client, ctx context.Context, tg homeassistant.Target) error {
			_, err := c.GetTriggersForTarget(ctx, tg, nil)
			return err
		},
		"conditions": func(c homeassistant.Client, ctx context.Context, tg homeassistant.Target) error {
			_, err := c.GetConditionsForTarget(ctx, tg, nil)
			return err
		},
		"services": func(c homeassistant.Client, ctx context.Context, tg homeassistant.Target) error {
			_, err := c.GetServicesForTarget(ctx, tg, nil)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			full := buildFake(true)
			full.targetErrFn = func(tg homeassistant.Target) error {
				return fmt.Errorf("call failed: %w", &homeassistant.APIError{
					StatusCode: http.StatusBadRequest, Message: "bad " + strings.Join(tg.EntityID, ","),
				})
			}
			w, _, ctx := wrapFake(t, hidingPolicy(t), full)
			err := call(w, ctx, homeassistant.Target{EntityID: hiddenIDs})
			if err == nil {
				t.Fatal("expected the inner error")
			}
			if strings.Contains(err.Error(), "ha_mcp_") {
				t.Errorf("error echoes twin ids: %q", err)
			}
			if !strings.Contains(err.Error(), "bad "+joined) {
				t.Errorf("error = %q, want the caller's ids swapped back in order", err)
			}
			var apiErr *homeassistant.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest || strings.Contains(apiErr.Message, "ha_mcp_") {
				t.Errorf("APIError status/message not preserved: %#v", apiErr)
			}
		})
	}
}

func TestTargetErrors_UnrelatedErrorUnchanged(t *testing.T) {
	full := buildFake(true)
	sentinel := errors.New("boom")
	full.targetErrFn = func(homeassistant.Target) error { return sentinel }
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	if _, err := w.ExtractFromTarget(ctx, homeassistant.Target{EntityID: []string{"light.kitchen"}}, nil); !errors.Is(err, sentinel) {
		t.Errorf("an error without twins must pass through untouched: %v", err)
	}
}

// Entity-backed media sources (camera, image): any form that is not a plain
// lower-case entity id counts as hidden under a hiding policy, so hidden and
// missing ids answer alike.
func TestBrowseMedia_OddEntityIDFormsAnswerMissing(t *testing.T) {
	odd := []string{
		"media-source://camera/camera%2Esecret",
		"media-source://camera/camera.secret#frag",
		"media-source://camera/camera.secret?x=1",
		"media-source://camera/Camera.Secret",
		"media-source://camera/camera.secret%20",
		"media-source://camera/camera.gone%20",
		"media-source://camera/Camera.Gone",
		"media-source://camera/camera%2Fgone",
		"media-source://camera/camera%zz",
		"media-source://image/image%2Esecret_pic",
		"media-source://image/Image.Pic",
		"Media-Source://Camera/camera.secret",
		"  media-source://camera/camera.secret  ",
	}
	full := buildFake(true)
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	const want = "browse media failed: command failed: browse_media_failed - Unknown item"
	for _, id := range odd {
		res, err := w.BrowseMedia(ctx, id)
		if err == nil || res != nil || err.Error() != want {
			t.Errorf("%q = %v, %v; want the missing-item error", id, res, err)
		}
	}
	if full.called("BrowseMedia") {
		t.Error("an odd id reached the inner client")
	}
	if tr.First() != nil {
		t.Error("recorded a denial")
	}
}

func TestBrowseMedia_OddEntityIDFormsDroppedFromTrees(t *testing.T) {
	full := buildFake(true)
	full.media = &homeassistant.MediaBrowseResult{Title: "root", Children: []*homeassistant.MediaBrowseResult{
		{Title: "enc", MediaContentID: "media-source://camera/camera%2Esecret"},
		{Title: "frag", MediaContentID: "media-source://camera/camera.secret#x"},
		{Title: "query", MediaContentID: "media-source://image/image.secret_pic?x=1"},
		{Title: "upper", MediaContentID: "media-source://camera/Camera.Front"},
		{Title: "ok", MediaContentID: "media-source://camera/camera.front"},
	}}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	got, err := w.BrowseMedia(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Children) != 1 || got.Children[0].Title != "ok" {
		t.Errorf("children = %+v", got.Children)
	}
}

// Sources without an entity mapping are never hidden, even under default none.
func TestBrowseMedia_OtherSourcesUntouched(t *testing.T) {
	cfg := defaultNonePolicy(t)
	full := buildFake(true)
	full.media = &homeassistant.MediaBrowseResult{Title: "root", Children: []*homeassistant.MediaBrowseResult{
		{Title: "local", MediaContentID: "media-source://media_source/local/music"},
		{Title: "tts", MediaContentID: "media-source://tts/cloud"},
		{Title: "radio", MediaContentID: "media-source://radio_browser/popular"},
		{Title: "camera folder", MediaContentID: "media-source://camera"},
		{Title: "camera secret", MediaContentID: "media-source://camera/camera.secret"},
	}}
	w, _, ctx := wrapFake(t, mustCompile(t, *cfg), full)
	got, err := w.BrowseMedia(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, c := range got.Children {
		titles = append(titles, c.Title)
	}
	if want := []string{"local", "tts", "radio", "camera folder"}; !slices.Equal(titles, want) {
		t.Errorf("children = %v, want %v", titles, want)
	}
	for _, id := range []string{"media-source://media_source/local/music", "media-source://tts/cloud", "media-source://radio_browser/popular"} {
		if _, err := w.BrowseMedia(ctx, id); err != nil {
			t.Errorf("browsing %s must stay possible: %v", id, err)
		}
	}
}
