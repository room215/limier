package adapters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/room215/limier/internal/adapter"
	"github.com/room215/limier/internal/adapters/cargo"
	"github.com/room215/limier/internal/adapters/npm"
	"github.com/room215/limier/internal/adapters/pip"
)

var registry = map[string]func() adapter.Adapter{
	"cargo": cargo.New,
	"npm":   npm.New,
	"pip":   pip.New,
}

func Lookup(name string) (adapter.Adapter, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	constructor, ok := registry[normalized]
	if !ok {
		return nil, fmt.Errorf("unsupported ecosystem %q; supported adapters: %s", name, strings.Join(Supported(), ", "))
	}

	return constructor(), nil
}

func Supported() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}
