// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imagefactory provides an abstraction to the image factory for the server.
//
// There are two implementations: Client talks to an image factory directly, and OmniClient goes through Omni,
// which picks the image factory it is configured with and authenticates against it, e.g., an Enterprise Image Factory.
package imagefactory

import (
	"errors"
	"time"

	"github.com/siderolabs/image-factory/pkg/schematic"
)

// requestTimeout bounds the image factory (or Omni) calls made to serve a single boot request.
const requestTimeout = 10 * time.Second

// IPXERequest describes the Talos boot assets to resolve the iPXE URL for.
type IPXERequest struct {
	// SchematicID is the ID of an existing schematic to boot. When set, Extensions and ExtraKernelArgs are not used.
	SchematicID     string
	TalosVersion    string
	Arch            string
	Extensions      []string
	ExtraKernelArgs []string
}

func (r IPXERequest) validate() error {
	if r.TalosVersion == "" {
		return errors.New("talos version is required")
	}

	if r.Arch == "" {
		return errors.New("arch is required")
	}

	return nil
}

func (r IPXERequest) schematic() schematic.Schematic {
	return schematic.Schematic{
		Customization: schematic.Customization{
			ExtraKernelArgs: r.ExtraKernelArgs,
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: r.Extensions,
			},
		},
	}
}
