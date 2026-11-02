// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/booter/internal/server/imagefactory"
)

func TestClientExistingSchematic(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expected   string
		secureBoot bool
	}{
		{name: "regular", expected: "https://pxe.factory.talos.dev/pxe/" + testSchematicID + "/v1.14.5/metal-amd64"},
		{name: "secure boot", secureBoot: true, expected: "https://pxe.factory.talos.dev/pxe/" + testSchematicID + "/v1.14.5/metal-amd64-secureboot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			factoryClient, err := imagefactory.NewClient("https://factory.talos.dev", "https://pxe.factory.talos.dev", tc.secureBoot, zaptest.NewLogger(t))
			require.NoError(t, err)

			// an existing schematic is not created, so this does not reach the image factory
			ipxeURL, err := factoryClient.IPXEURL(t.Context(), imagefactory.IPXERequest{
				SchematicID:  testSchematicID,
				TalosVersion: "v1.14.5",
				Arch:         "amd64",
			})
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ipxeURL)
		})
	}
}
