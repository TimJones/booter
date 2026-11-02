// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	omniconstants "github.com/siderolabs/omni/client/pkg/constants"
	omniresources "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/booter/internal/server/omni"
)

func TestDefaultTalosVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for _, version := range []string{"1.13.10", "1.14.0", "1.14.1"} {
		talosVersion := omniresources.NewTalosVersion(version)
		talosVersion.TypedSpec().Value.Version = version

		require.NoError(t, st.Create(ctx, talosVersion))
	}

	// an Omni that labels no version as its default
	version, err := omni.DefaultTalosVersion(ctx, st)
	require.NoError(t, err)
	assert.Equal(t, omniconstants.DefaultTalosVersion, version)

	_, err = safe.StateUpdateWithConflicts(ctx, st, omniresources.NewTalosVersion("1.14.0").Metadata(), func(res *omniresources.TalosVersion) error {
		res.Metadata().Labels().Set(omniresources.LabelDefaultVersion, "")

		return nil
	})
	require.NoError(t, err)

	version, err = omni.DefaultTalosVersion(ctx, st)
	require.NoError(t, err)
	assert.Equal(t, "1.14.0", version)
}
