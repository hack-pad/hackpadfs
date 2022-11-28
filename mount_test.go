package hackpadfs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestStripErrPathPrefix(t *testing.T) {
	t.Parallel()
	someError := errors.New("some error")
	for _, tc := range []struct {
		err          error
		name         string
		mountSubPath string
		expectErr    error
	}{
		{
			err:       nil,
			expectErr: nil,
		},
		{
			err:       someError,
			expectErr: someError,
		},
		{
			err: &PathError{
				Op:   "foo",
				Path: "biff/baz/bar",
				Err:  someError,
			},
			name:         "baz/bar",
			mountSubPath: "biff/baz/bar",
			expectErr: &PathError{
				Op:   "foo",
				Path: "baz/bar",
				Err:  someError,
			},
		},
		{
			err: &PathError{
				Op:   "foo",
				Path: "biff/baz/bar",
				Err:  someError,
			},
			name:         "biff/baz/bar",
			mountSubPath: "biff/baz/bar",
			expectErr: &PathError{
				Op:   "foo",
				Path: "biff/baz/bar",
				Err:  someError,
			},
		},
		{
			err: &LinkError{
				Op:  "foo",
				Old: "biff/baz/bar",
				New: "biff/baz/bat",
				Err: someError,
			},
			name:         "baz/bar",
			mountSubPath: "biff/baz/bar",
			expectErr: &LinkError{
				Op:  "foo",
				Old: "baz/bar",
				New: "baz/bat",
				Err: someError,
			},
		},
	} {
		tc := tc // enable parallel sub-tests
		t.Run(fmt.Sprint(tc.err, tc.name, tc.mountSubPath), func(t *testing.T) {
			t.Parallel()
			err := stripErrPathPrefix(tc.err, tc.name, tc.mountSubPath)
			assert.Equal(t, tc.expectErr, err)
		})
	}
}
