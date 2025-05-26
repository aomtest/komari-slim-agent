package monitoring

import (
	"testing"
)

func TestVirtualized(t *testing.T) {
	virt := Virtualized()
	t.Logf("Virtualization type: %s", virt)
}
