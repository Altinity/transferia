package variant

import (
	"os"
	"strings"
)

const EnvSourceVariant = "SOURCE_VARIANT"

// Selection stores parsed SOURCE_VARIANT in form "family/variant".
type Selection struct {
	Family  string
	Variant string
}

func Current() Selection {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(EnvSourceVariant)))
	if raw == "" {
		return Selection{}
	}
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		return Selection{}
	}
	family := strings.TrimSpace(parts[0])
	variant := strings.TrimSpace(parts[1])
	if family == "" || variant == "" {
		return Selection{}
	}
	return Selection{
		Family:  family,
		Variant: variant,
	}
}

func ForFamily(family string) (string, bool) {
	sel := Current()
	if sel.Family == "" || sel.Variant == "" {
		return "", false
	}
	if sel.Family != strings.ToLower(strings.TrimSpace(family)) {
		return "", false
	}
	return sel.Variant, true
}
