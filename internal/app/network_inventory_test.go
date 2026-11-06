package app

import (
	"reflect"
	"testing"

	"github.com/adnanh/webhook/internal/hook"
)

func TestMandatoryIPRangesOnlyFromConjunctions(t *testing.T) {
	allow := hook.Rules{Match: &hook.MatchRule{Type: hook.IPWhitelist, IPRange: "192.0.2.0/24"}}
	other := hook.Rules{Match: &hook.MatchRule{Type: hook.MatchValue, Value: "ok"}}
	and := hook.AndRule{allow, hook.Rules{Or: &hook.OrRule{allow, other}}}
	if got := mandatoryIPRanges(&hook.Rules{And: &and}); !reflect.DeepEqual(got, []string{"192.0.2.0/24"}) {
		t.Fatalf("required ranges = %v", got)
	}
	or := hook.OrRule{allow, other}
	if got := mandatoryIPRanges(&hook.Rules{Or: &or}); len(got) != 0 {
		t.Fatalf("OR rule should not produce a required IP range: %v", got)
	}
}
