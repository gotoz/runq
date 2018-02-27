package main

import (
	"fmt"
	"strings"

	"github.com/gotoz/runq/pkg/vm"
	"github.com/pkg/errors"
	"github.com/syndtr/gocapability/capability"
)

// capMap stores all available capabilities.
var capMap map[string]capability.Cap

func init() {
	capMap = make(map[string]capability.Cap)
	for _, v := range capability.List() {
		if v > capability.CAP_LAST_CAP {
			continue
		}
		k := fmt.Sprintf("CAP_%s", strings.ToUpper(v.String()))
		capMap[k] = v
	}
}

// listToCap converts list of capability strings into capability types.
func listToCap(list []string) ([]capability.Cap, error) {
	var caps []capability.Cap
	for _, v := range list {
		c, ok := capMap[v]
		if !ok {
			return nil, errors.Errorf("unknown capability %q", v)
		}
		caps = append(caps, c)
	}
	return caps, nil
}

func dropCapabilities(vmcaps vm.AppCapabilities) error {
	const allCapTypes = capability.CAPS | capability.BOUNDS

	p, err := capability.NewPid(0)
	if err != nil {
		return errors.WithStack(err)
	}
	p.Clear(allCapTypes)

	for capType, list := range map[capability.CapType][]string{
		capability.BOUNDS:      vmcaps.Bounding,
		capability.EFFECTIVE:   vmcaps.Effective,
		capability.INHERITABLE: vmcaps.Inheritable,
		capability.PERMITTED:   vmcaps.Permitted,
	} {
		caps, err := listToCap(list)
		if err != nil {
			return err
		}
		p.Set(capType, caps...)
	}

	err = p.Apply(allCapTypes)
	return errors.WithStack(err)
}
