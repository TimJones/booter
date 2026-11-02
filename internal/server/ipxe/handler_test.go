// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ipxe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/booter/internal/server/imagefactory"
	"github.com/siderolabs/booter/internal/server/ipxe"
)

// TestNewHandlerPatchesEmbeddedBinaries checks that the iPXE binaries booter embeds, from the iPXE image pinned
// in its .kres.yaml, can be patched by the shared iPXE package, which is tested against the provider's binaries.
func TestNewHandlerPatchesEmbeddedBinaries(t *testing.T) {
	t.Parallel()

	handler, err := ipxe.NewHandler(false, nil, ipxe.HandlerOptions{
		APIAdvertiseAddress: "192.168.1.100",
		APIPort:             8081,
		TalosVersion:        "v1.13.0",
	}, zaptest.NewLogger(t))
	require.NoError(t, err)

	require.NotEmpty(t, handler.PatchedFiles()["snp.efi"])
	require.NotEmpty(t, handler.PatchedFiles()["undionly.kpxe"])
}

type fakeImageFactoryClient struct {
	requests []imagefactory.IPXERequest
}

func (f *fakeImageFactoryClient) IPXEURL(_ context.Context, request imagefactory.IPXERequest) (string, error) {
	f.requests = append(f.requests, request)

	return "https://factory.example/pxe/some-schematic/v1.14.5/metal-" + request.Arch + "?token=download-token", nil
}

func TestHandlerServesFactoryIPXEScript(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		expected imagefactory.IPXERequest
		options  ipxe.HandlerOptions
	}{
		{
			name: "generated schematic",
			options: ipxe.HandlerOptions{
				Extensions:      []string{"siderolabs/iscsi-tools"},
				ExtraKernelArgs: "foo=bar",
			},
			expected: imagefactory.IPXERequest{
				TalosVersion:    "v1.14.5",
				Arch:            "arm64",
				Extensions:      []string{"siderolabs/iscsi-tools"},
				ExtraKernelArgs: []string{"foo=bar", "talos.config=http://192.168.1.100:8081/config?u=${uuid}", "console=tty0", "console=ttyAMA0"},
			},
		},
		{
			name: "existing schematic",
			options: ipxe.HandlerOptions{
				SchematicID: "some-schematic",
			},
			expected: imagefactory.IPXERequest{
				SchematicID:  "some-schematic",
				TalosVersion: "v1.14.5",
				Arch:         "arm64",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			factoryClient := &fakeImageFactoryClient{}

			tc.options.APIAdvertiseAddress = "192.168.1.100"
			tc.options.APIPort = 8081
			tc.options.TalosVersion = "v1.14.5"

			handler, err := ipxe.NewHandler(true, factoryClient, tc.options, zaptest.NewLogger(t))
			require.NoError(t, err)

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ipxe/boot.ipxe?arch=arm64", nil))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "#!ipxe\nchain --replace https://factory.example/pxe/some-schematic/v1.14.5/metal-arm64?token=download-token\n", recorder.Body.String())

			require.Len(t, factoryClient.requests, 1)
			assert.Equal(t, tc.expected, factoryClient.requests[0])
		})
	}
}
