package main

import (
	"sort"
	"strings"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/adnanh/webhook/internal/network"
)

func networkHookInventory() []network.HookInfo {
	loadedHooksMu.RLock()
	defer loadedHooksMu.RUnlock()
	var result []network.HookInfo
	for _, hooksInFile := range loadedHooksFromFiles {
		for _, current := range hooksInFile {
			if !network.ValidHookID(current.ID) {
				continue
			}
			methods := current.HTTPMethods
			if len(methods) == 0 && *httpMethods != "" {
				methods = strings.Split(*httpMethods, ",")
			}
			info := network.HookInfo{ID: current.ID}
			for _, method := range methods {
				method = strings.ToUpper(strings.TrimSpace(method))
				if method != "" {
					info.Methods = append(info.Methods, method)
				}
			}
			sort.Strings(info.Methods)
			info.IPRequired = mandatoryIPRanges(current.TriggerRule)
			info.AllowIPs = splitIPList(*accessWhitelist)
			info.DenyIPs = splitIPList(*accessBlacklist)
			result = append(result, info)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func splitIPList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
}

func mandatoryIPRanges(rule *hook.Rules) []string {
	if rule == nil {
		return nil
	}
	switch {
	case rule.And != nil:
		var ranges []string
		for i := range *rule.And {
			ranges = append(ranges, mandatoryIPRanges(&(*rule.And)[i])...)
		}
		return ranges
	case rule.Or != nil || rule.Not != nil:
		return nil
	case rule.Match != nil && rule.Match.Type == hook.IPWhitelist && strings.TrimSpace(rule.Match.IPRange) != "":
		if _, err := hook.CheckIPWhitelist("127.0.0.1", rule.Match.IPRange); err != nil {
			return nil
		}
		return []string{rule.Match.IPRange}
	default:
		return nil
	}
}
