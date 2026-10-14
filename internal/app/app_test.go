package app

import (
	"testing"

	"go.uber.org/fx"
)

func TestModuleGraph(t *testing.T) {
	if err := fx.ValidateApp(Options()...); err != nil {
		t.Fatal(err)
	}
}
