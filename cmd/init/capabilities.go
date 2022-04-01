package main

import (
	"fmt"
	"strings"

	"github.com/gotoz/runq/pkg/vm"
	"github.com/pkg/errors"
	"github.com/syndtr/gocapability/capability"
)

func dropCapabilities(vmcaps vm.AppCapabilities) error {
	// capMap stores all available capabilities.
	capMap := make(map[string]capability.Cap)
	for _, v := range capability.List() {
		if v > capability.CAP_LAST_CAP {
			continue
		}
		k := fmt.Sprintf("CAP_%s", strings.ToUpper(v.String()))
		capMap[k] = v
	}

	// listToCap converts list of capability strings into capability types.
	listToCap := func(list []string) ([]capability.Cap, error) {
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

	minCaps := []string{"CAP_SETGID", "CAP_SETUID", "CAP_SYS_ADMIN"}

	p, err := capability.NewPid2(0)
	if err != nil {
		return errors.WithStack(err)
	}
	err = p.Load()
	if err != nil {
		return errors.WithStack(err)
	}

	p.Clear(capability.CAPS | capability.BOUNDS)

	for capType, list := range map[capability.CapType][]string{
		capability.EFFECTIVE:   minCaps,
		capability.PERMITTED:   minCaps,
		capability.BOUNDS:      vmcaps.Bounding,
		capability.INHERITABLE: vmcaps.Inheritable,
	} {
		caps, err := listToCap(list)
		if err != nil {
			return err
		}
		p.Set(capType, caps...)
	}

	err = p.Apply(capability.CAPS | capability.BOUNDS)
	return errors.WithStack(err)
}
