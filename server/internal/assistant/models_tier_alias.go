package assistant

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/nwverse/persona/server/internal/assistantwire"
	"github.com/nwverse/persona/server/internal/i18n"
)

// apiTierAliases: the Claude Code tier aliases a model field can hold (the picker offers them for a launcher's CLI,
// whose node resolves them through ANTHROPIC_DEFAULT_*_MODEL, node/launcherenv.go resolveTier), and which of a
// launcher's tier fields each runs. As on a computer (node's providerEnv), haiku falls back to the small model and
// best runs what opus does (Claude Code 2.1.321); opusplan is opus's tier too.
var apiTierAliases = map[string]func(storedLauncher) string{
	"opus":     func(l storedLauncher) string { return l.OpusModel },
	"best":     func(l storedLauncher) string { return l.OpusModel },
	"opusplan": func(l storedLauncher) string { return l.OpusModel },
	"sonnet":   func(l storedLauncher) string { return l.SonnetModel },
	"haiku":    func(l storedLauncher) string { return cmp.Or(l.HaikuModel, l.SmallModel) },
	"fable":    func(l storedLauncher) string { return l.FableModel },
}

// apiTierAliasKey is m's entry in apiTierAliases ("Sonnet", "opus[1m]" alike), and whether it has one.
func apiTierAliasKey(m string) (string, bool) {
	k := strings.ToLower(strings.TrimSpace(m))
	k = strings.TrimSuffix(k, "[1m]")
	_, ok := apiTierAliases[k]
	return k, ok
}

// resolveAPITierAlias is model as a direct API call to l's gateway sends it (#9165): a tier alias becomes the model
// l's tier field names, since no gateway serves a model called "sonnet"; any other model as it is. A tier alias l
// names no model for (or names another alias for) is an error, not a request the gateway refuses with a vague 4xx.
func resolveAPITierAlias(l storedLauncher, model string) (string, error) {
	k, ok := apiTierAliasKey(model)
	if !ok {
		return model, nil
	}
	if v := strings.TrimSpace(apiTierAliases[k](l)); v != "" {
		if _, alias := apiTierAliasKey(v); !alias {
			return v, nil
		}
	}
	return "", i18n.Err("assistant-models.err_tier_alias_unresolved", i18n.Args{"model": model, "provider": l.Name})
}

// migrateTierAliases keeps existing bindings callable when a Chat save removes the tier fields they use. The caller
// holds models.mu and writes the returned records with next in one transaction. Haiku's small-model fallback is an
// alias mapping too: replacing API launch fields must not strand a purpose that names haiku.
func (s *Service) migrateTierAliases(ctx context.Context, old storedLauncher, next *storedLauncher) (map[string]any, error) {
	moved := map[string]string{}
	for alias := range apiTierAliases {
		before, err := resolveAPITierAlias(old, alias)
		if err != nil {
			continue
		}
		after, afterErr := resolveAPITierAlias(*next, alias)
		if afterErr != nil || after != before {
			moved[alias] = before
		}
	}
	if len(moved) == 0 {
		return nil, nil
	}
	if alias, ok := apiTierAliasKey(next.Model); ok && moved[alias] != "" {
		next.Model = moved[alias]
	}

	provider := launcherProvider(old.Name)
	providers := s.modelProviders(ctx)
	ids := slices.Clone(modelConfigIDs)
	for _, x := range s.modelExtensions(ctx) {
		ids = append(ids, assistantwire.ExtensionPurposePrefix+x.StorageKey())
	}
	writes := map[string]any{}
	for _, id := range ids {
		p, err := s.modelPurpose(ctx, id, providers)
		if err != nil {
			return nil, err
		}
		alias, ok := apiTierAliasKey(p.Override.Model)
		if p.Effective.Provider != provider || !ok || moved[alias] == "" {
			continue
		}
		binding := p.Override
		binding.Model = moved[alias]
		writes[modelRecordKey(modelRecordID(p))] = modelRecord{Revision: p.Revision + 1, Binding: binding}
	}
	return writes, nil
}
