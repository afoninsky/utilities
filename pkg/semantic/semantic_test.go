package semantic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	var err error

	r, err := New("")
	assert.NoError(t, err)

	info, err := r.Info()
	assert.NoError(t, err)
	fmt.Println("current version:", info.LatestVersion)
	fmt.Println("current tag:", info.CurrentTag)
	fmt.Println("next version:", info.NextVersion)
}
