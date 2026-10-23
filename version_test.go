package pele_test

import (
	"testing"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/internal/oracle"
)

// TestUpstreamVersionMatchesOracle keeps the release the package claims to match in step
// with the bundle parity is checked against, so a release bump can't change one alone.
func TestUpstreamVersionMatchesOracle(t *testing.T) {
	if pele.UpstreamVersion != oracle.Version {
		t.Errorf("pele.UpstreamVersion is %q but the oracle bundle is %q", pele.UpstreamVersion, oracle.Version)
	}
}
